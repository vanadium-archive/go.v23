// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test/v23tests"
)

//go:generate jiri test generate

var (
	hostname   string
	errTimeout = errors.New("timeout")
)

func init() {
	name, err := os.Hostname()
	if err != nil {
		panic(fmt.Sprintf("Hostname() failed: %v", err))
	}
	hostname = name
}

// V23TestDeviceManager publishes and runs syncbased using the device manager
// and verifies that the running instance of syncbased has appropriate
// blessings.
func V23TestDeviceManager(i *v23tests.T) {
	defer fmt.Fprintf(os.Stderr, "--------------- SHUTDOWN ---------------\n")
	var (
		workDir       = i.NewTempDir("")
		binStagingDir = mkSubdir(i, workDir, "bin")
		dmInstallDir  = filepath.Join(workDir, "dm")

		// Most vanadium command-line utilities will be run by a
		// principal that has "root/u/alice" as its blessing.
		// (Where "root" comes from i.Principal().BlessingStore().Default()).
		// Create those credentials and options to use to setup the
		// binaries with them.
		aliceCreds, _ = i.Shell().NewChildCredentials("u/alice")
		aliceOpts     = i.Shell().DefaultStartOpts().ExternalProgram().WithCustomCredentials(aliceCreds)

		// Build all the command-line tools and set them up to run as alice.
		// applicationd/binaryd servers will be run by alice too.
		// TODO: applicationd/binaryd should run as a separate "service" role, as
		// alice is just a user.
		namespaceBin    = i.BuildV23Pkg("v.io/x/ref/cmd/namespace").WithStartOpts(aliceOpts)
		deviceBin       = i.BuildV23Pkg("v.io/x/ref/services/device/device").WithStartOpts(aliceOpts)
		binarydBin      = i.BuildV23Pkg("v.io/x/ref/services/binary/binaryd").WithStartOpts(aliceOpts)
		applicationdBin = i.BuildV23Pkg("v.io/x/ref/services/application/applicationd").WithStartOpts(aliceOpts)
		syncbasedBin    = i.BuildV23Pkg("v.io/x/ref/services/syncbase/syncbased")

		// The devicex script is not provided with any credentials, it
		// will generate its own.  This means that on "devicex start"
		// the device will have no useful credentials and until "device
		// claim" is invoked (as alice), it will just sit around
		// waiting to be claimed.
		//
		// Other binaries, like applicationd and binaryd will be run by alice.
		deviceScriptPath = filepath.Join(os.Getenv("JIRI_ROOT"), "release", "go", "src", "v.io", "x", "ref", "services", "device", "devicex")
		deviceScript     = i.BinaryFromPath(deviceScriptPath).WithEnv("V23_DEVICE_DIR=" + dmInstallDir)

		mtName = "devices/" + hostname // Name under which the device manager will publish itself.
	)

	// We also need some tools running with different sets of credentials...

	// Administration tasks will be performed with a blessing that represents a corporate
	// administrator (which is usually a role account)
	adminCreds, err := i.Shell().NewChildCredentials("r/admin")
	if err != nil {
		i.Fatalf("generating admin creds: %v", err)
	}
	adminOpts := i.Shell().DefaultStartOpts().ExternalProgram().WithCustomCredentials(adminCreds)
	adminDeviceBin := deviceBin.WithStartOpts(adminOpts)
	debugBin := i.BuildV23Pkg("v.io/x/ref/services/debug/debug").WithStartOpts(adminOpts)

	// A special set of credentials will be used to give two blessings to the device manager
	// when claiming it -- one blessing will be from the corporate administrator role who owns
	// the machine, and the other will be a manufacturer blessing. (This is a hack until
	// there's a way to separately supply a manufacturer blessing. Eventually, the claim
	// would really be done by the administrator, and the administrator's blessing would get
	// added to the manufacturer's blessing, which would already be present.)
	claimCreds, err := i.Shell().AddToChildCredentials(adminCreds, "m/orange/zphone5/ime-i007")
	if err != nil {
		i.Fatalf("adding the mfr blessing to admin creds: %v", err)
	}
	claimOpts := i.Shell().DefaultStartOpts().ExternalProgram().WithCustomCredentials(claimCreds)
	claimDeviceBin := deviceBin.WithStartOpts(claimOpts)

	// Another set of credentials be used to represent the application publisher, who
	// signs and pushes binaries
	pubCreds, err := i.Shell().NewChildCredentials("a/rovio")
	if err != nil {
		i.Fatalf("generating publisher creds: %v", err)
	}
	pubOpts := i.Shell().DefaultStartOpts().ExternalProgram().WithCustomCredentials(pubCreds)
	pubDeviceBin := deviceBin.WithStartOpts(pubOpts)
	applicationBin := i.BuildV23Pkg("v.io/x/ref/services/application/application").WithStartOpts(pubOpts)

	v23tests.RunRootMT(i, "--v23.tcp.address=127.0.0.1:0")
	buildAndCopyBinaries(
		i,
		binStagingDir,
		"v.io/x/ref/services/device/deviced",
		"v.io/x/ref/services/agent/agentd",
		"v.io/x/ref/services/device/suidhelper",
		"v.io/x/ref/services/device/inithelper")

	appDName := "applications"
	devicedAppName := filepath.Join(appDName, "deviced", "test")

	deviceScript.Start(
		"install",
		binStagingDir,
		"--single_user",
		"--origin="+devicedAppName,
		"--",
		"--v23.tcp.address=127.0.0.1:0",
		"--neighborhood-name="+fmt.Sprintf("%s-%d-%d", hostname, os.Getpid(), rand.Int()),
	).WaitOrDie(os.Stdout, os.Stderr)
	deviceScript.Start("start").WaitOrDie(os.Stdout, os.Stderr)
	// Grab the endpoint for the claimable service from the device manager's
	// log.
	dmLog := filepath.Join(dmInstallDir, "dmroot/device-manager/logs/deviced.INFO")
	var claimableEP string
	expiry := time.Now().Add(30 * time.Second)
	for {
		if time.Now().After(expiry) {
			i.Fatalf("Timed out looking for claimable endpoint in %v", dmLog)
		}
		startLog, err := ioutil.ReadFile(dmLog)
		if err != nil {
			i.Logf("Couldn't read log %v: %v", dmLog, err)
			time.Sleep(time.Second)
			continue
		}
		re := regexp.MustCompile(`Unclaimed device manager \((.*)\)`)
		matches := re.FindSubmatch(startLog)
		if len(matches) == 0 {
			i.Logf("Couldn't find match in %v [%v]", dmLog, startLog)
			time.Sleep(time.Second)
			continue
		}
		if len(matches) != 2 {
			i.Fatalf("Wrong match in %v (%d) %v", dmLog, len(matches), string(matches[0]))
		}
		claimableEP = string(matches[1])
		break
	}
	// Claim the device as "root/u/alice/myworkstation".
	claimDeviceBin.Start("claim", claimableEP, "myworkstation")

	resolve := func(name string) string {
		resolver := func() (interface{}, error) {
			// Use Start, rather than Run, since it's ok for 'namespace resolve'
			// to fail with 'name doesn't exist'
			inv := namespaceBin.Start("resolve", name)
			// Cleanup after ourselves to avoid leaving a ton of invocations
			// lying around which obscure logging output.
			defer inv.Wait(nil, os.Stderr)
			if r := strings.TrimRight(inv.Output(), "\n"); len(r) > 0 {
				return r, nil
			}
			return nil, nil
		}
		return i.WaitFor(resolver, 100*time.Millisecond, time.Minute).(string)
	}

	// Wait for the device manager to publish its mount table entry.
	resolve(mtName)
	adminDeviceBin.Run("acl", "set", mtName+"/devmgr/device", "root/u/alice", "Read,Resolve,Write")

	// Verify the device's default blessing is as expected.
	mfrBlessing := "root/m/orange/zphone5/ime-i007/myworkstation"
	ownerBlessing := "root/r/admin/myworkstation"
	inv := debugBin.Start("stats", "read", mtName+"/devmgr/__debug/stats/security/principal/*/blessingstore")
	inv.ExpectSetEventuallyRE(".*Default Blessings[ ]+" + mfrBlessing + "," + ownerBlessing)

	// Get the device's profile, which should be set to non-empty string
	inv = adminDeviceBin.Start("describe", mtName+"/devmgr/device")

	parts := inv.ExpectRE(`{Profiles:map\[(.*):{}\]}`, 1)
	expectOneMatch := func(parts [][]string) string {
		if len(parts) != 1 || len(parts[0]) != 2 {
			loc := v23tests.Caller(1)
			i.Fatalf("%s: failed to match profile: %#v", loc, parts)
		}
		return parts[0][1]
	}
	deviceProfile := expectOneMatch(parts)
	if len(deviceProfile) == 0 {
		i.Fatalf("failed to get profile")
	}

	// Start a binaryd server that will serve the binary for the test
	// application to be installed on the device.
	binarydName := "binaries"
	binarydBin.Start(
		"--name=binaries",
		"--root-dir="+filepath.Join(workDir, "binstore"),
		"--v23.tcp.address=127.0.0.1:0",
		"--http=127.0.0.1:0")
	// Allow publishers to update binaries
	deviceBin.Run("acl", "set", binarydName, "root/a", "Write")

	// Start an applicationd server that will serve the application
	// envelope for the test application to be installed on the device.
	applicationdBin.Start(
		"--name="+appDName,
		"--store="+mkSubdir(i, workDir, "appstore"),
		"--v23.tcp.address=127.0.0.1:0",
	)
	// Allow publishers to create and update envelopes
	deviceBin.Run("acl", "set", appDName, "root/a", "Read,Write,Resolve")

	syncbasedName := appDName + "/syncbased"
	syncbasedBinName := binarydName + "/syncbased"
	syncbasedEnvelopeFilename := filepath.Join(workDir, "syncbased.envelope")
	syncbasedEnvelope := fmt.Sprintf("{\"Title\":\"syncbased\","+
		"\"Args\":[\"-v=0\", \"--name=syncbased\", \"--root-dir=%s\", \"--v23.tcp.address=127.0.0.1:0\"]}",
		mkSubdir(i, workDir, "syncbased"))
	ioutil.WriteFile(syncbasedEnvelopeFilename, []byte(syncbasedEnvelope), 0666)
	defer os.Remove(syncbasedEnvelopeFilename)

	output := applicationBin.Run("put", syncbasedName+"/0", deviceProfile, syncbasedEnvelopeFilename)
	if got, want := output, fmt.Sprintf("Application envelope added for profile %s.", deviceProfile); got != want {
		i.Fatalf("got %q, want %q", got, want)
	}

	// Publish the app.
	pubDeviceBin.Start("publish", "-from", filepath.Dir(syncbasedBin.Path()), "-readers", "root/r/admin", "syncbased").WaitOrDie(os.Stdout, os.Stderr)
	if got := namespaceBin.Run("glob", syncbasedBinName); len(got) == 0 {
		i.Fatalf("glob failed for %q", syncbasedBinName)
	}

	// Install the app on the device.
	inv = deviceBin.Start("install", mtName+"/devmgr/apps", syncbasedName)
	installationName := inv.ReadLine()
	if installationName == "" {
		i.Fatalf("got empty installation name from install")
	}

	// Verify that the installation shows up when globbing the device manager.
	output = namespaceBin.Run("glob", mtName+"/devmgr/apps/syncbased/*")
	if got, want := output, installationName; got != want {
		i.Fatalf("got %q, want %q", got, want)
	}

	// Start an instance of the app, granting it blessing extension syncbased.
	inv = deviceBin.Start("instantiate", installationName, "syncbased")
	instanceName := inv.ReadLine()
	if instanceName == "" {
		i.Fatalf("got empty instance name from new")
	}
	deviceBin.Start("run", instanceName).Wait(os.Stdout, os.Stderr)

	resolve(mtName + "/syncbased")

	// Verify that the instance shows up when globbing the device manager.
	output = namespaceBin.Run("glob", mtName+"/devmgr/apps/syncbased/*/*")
	if got, want := output, instanceName; got != want {
		i.Fatalf("got %q, want %q", got, want)
	}

	// Verify the app's blessings. We check the default blessing, as well as the
	// "..." blessing, which should be the default blessing plus a publisher blessing.
	userBlessing := "root/u/alice/syncbased"
	pubBlessing := "root/a/rovio/apps/published/syncbased"
	// Just to remind:
	// mfrBlessing   = "root/m/orange/zphone5/ime-i007/myworkstation"
	// ownerBlessing = "root/r/admin/myworkstation"
	appBlessing := mfrBlessing + "/a/" + pubBlessing + "," + ownerBlessing + "/a/" + pubBlessing
	inv = debugBin.Start("stats", "read", instanceName+"/stats/security/principal/*/blessingstore")
	inv.ExpectSetEventuallyRE(".*Default Blessings[ ]+"+userBlessing+"$", "[.][.][.][ ]+"+userBlessing+","+appBlessing)

	// Kill and delete the instance.
	deviceBin.Run("kill", instanceName)
	deviceBin.Run("delete", instanceName)

	// Shut down the device manager.
	deviceScript.Run("stop")

	// Wait for the mounttable entry to go away.
	resolveGone := func(name string) string {
		resolver := func() (interface{}, error) {
			inv := namespaceBin.Start("resolve", name)
			defer inv.Wait(nil, os.Stderr)
			if r := strings.TrimRight(inv.Output(), "\n"); len(r) == 0 {
				return r, nil
			}
			return nil, nil
		}
		return i.WaitFor(resolver, 100*time.Millisecond, time.Minute).(string)
	}
	resolveGone(mtName)
	deviceScript.Run("uninstall")
}

func buildAndCopyBinaries(i *v23tests.T, destinationDir string, packages ...string) {
	var args []string
	for _, pkg := range packages {
		args = append(args, i.BuildGoPkg(pkg).Path())
	}
	args = append(args, destinationDir)
	i.BinaryFromPath("/bin/cp").Start(args...).WaitOrDie(os.Stdout, os.Stderr)
}

func mkSubdir(i *v23tests.T, parent, child string) string {
	dir := filepath.Join(parent, child)
	if err := os.Mkdir(dir, 0755); err != nil {
		i.Fatalf("failed to create %q: %v", dir, err)
	}
	return dir
}
