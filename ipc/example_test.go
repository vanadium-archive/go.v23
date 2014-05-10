package ipc_test

import (
	"fmt"
	"time"

	"veyron2"
	"veyron2/ipc"
	"veyron2/security"
)

type photoServer struct{ suffix string }

func (ps *photoServer) Lookup(suffix string) (ipc.Invoker, security.Authorizer, error) {
	return ipc.ReflectInvoker(&Photo{ps.suffix}), nil, nil
}

type Photo struct{ Name string }

func (p *Photo) Get(call ipc.ServerCall, _ interface{}, photo *Photo) error {
	*photo = *p
	return nil
}

func (*Photo) Download(call ipc.ServerCall) error {
	// Use suffix as a regexp to obtain a list of photos
	// and stream each one back.
	photos := []string{"a.jpg", "b.jpg", "c.jpg"}
	for _, p := range photos {
		if call.IsClosed() {
			break
		}
		call.Send(&Photo{p})
	}
	return nil
}

func (*Photo) Upload(call ipc.ServerCall) error {
	for {
		if call.IsClosed() {
			break
		}
		var photo Photo
		if err := call.Recv(&photo); err != nil {
			break
		}
		// Do something with the photo
	}
	return nil
}

func Example_server() {
	jpegServer := &photoServer{}
	sm, err := RT.NewServer(veyron2.LocalID(security.FakePrivateID("server")))
	err = sm.Register("photos", jpegServer)
	_, err = sm.Listen("tcp", "127.0.0.1:0")
	err = sm.Publish("/myhome")
	time.Sleep(60 * time.Second)
	err = sm.Stop()
	_ = err
}

func Example_client() {
	client, err := RT.NewClient()
	defer client.Close()
	var photo Photo
	call, err := client.StartCall("/myhome/photos/birthday.jpg", "Get", nil)
	err = call.Finish(&photo)
	_ = err
}

func Example_clientCancel() {
	client, err := RT.NewClient()
	defer client.Close()
	var photo Photo
	call, err := client.StartCall("/myhome/photos/birthday.jpg", "Get", nil)
	call.Cancel()
	err = call.Finish(&photo)
	_ = err
}

func Example_clientStreamRecv() {
	client, err := RT.NewClient()
	defer client.Close()
	call, err := client.StartCall(`/myhome/photos/.*\.jpg`, "Download", nil)
	err = call.CloseSend()
	for {
		var photo Photo
		if err := call.Recv(&photo); err != nil {
			break
		}
		fmt.Printf("Photo: %s\n", photo.Name)
	}
	err = call.Finish()
	_ = err
}

func Example_clientStreamSend() {
	client, err := RT.NewClient()
	defer client.Close()
	call, err := client.StartCall(`/myhome/photos/newdir`, "Upload", nil)
	for _, np := range []string{"n1", "n2", "n3"} {
		if err := call.Send(&Photo{Name: np}); err != nil {
			break
		}
		fmt.Printf("Photo: %s\n", np)
	}
	err = call.CloseSend()
	err = call.Finish()
	_ = err
}
