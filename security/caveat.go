package security

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"time"

	"v.io/core/veyron2/uniqueid"
	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/vlog"
	"v.io/core/veyron2/vom"
)

type registryEntry struct {
	desc        CaveatDescriptor
	validatorFn reflect.Value
	paramType   reflect.Type
	registerer  string
}

// Instance of unconstrained use caveat, to be used by UnconstrainedCaveat().
var unconstrainedUseCaveat Caveat

func init() {
	var err error
	unconstrainedUseCaveat, err = NewCaveat(ConstCaveat, true)
	if err != nil {
		panic(fmt.Sprintf("Error in NewCaveat: %v", err))
	}
}

// caveatRegistry is used to implement a singleton global registry that maps
// the unique id of a caveat to its validation function.
//
// It is safe to invoke methods on caveatRegistry concurrently.
type caveatRegistry struct {
	mu     sync.RWMutex
	byUUID map[uniqueid.Id]registryEntry
}

var registry = &caveatRegistry{byUUID: make(map[uniqueid.Id]registryEntry)}

func (r *caveatRegistry) register(d CaveatDescriptor, validator interface{}) error {
	_, file, line, _ := runtime.Caller(2) // one for r.register, one for RegisterCaveatValidator
	registerer := fmt.Sprintf("%s:%d", file, line)
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, exists := r.byUUID[d.Id]; exists {
		return fmt.Errorf("Caveat with UUID %v registered twice. Once with (%v, fn=%p) from %v, once with (%v, fn=%p) from %v", d.Id, e.desc.ParamType, e.validatorFn.Interface(), e.registerer, d.ParamType, validator, registerer)
	}
	fn := reflect.ValueOf(validator)
	param := vdl.TypeToReflect(d.ParamType)
	if param == nil {
		// If you hit this error, https://github.com/veyron/release-issues/issues/907
		// might be the problem.
		return fmt.Errorf("invalid caveat descriptor: vdl.Type(%v) cannot be converted to a Go type", d.ParamType)
	}
	var (
		rtErr     = reflect.TypeOf((*error)(nil)).Elem()
		rtContext = reflect.TypeOf((*Context)(nil)).Elem()
	)
	if got, want := fn.Kind(), reflect.Func; got != want {
		return fmt.Errorf("invalid caveat validator: must be %v, not %v", want, got)
	}
	if got, want := fn.Type().NumOut(), 1; got != want {
		return fmt.Errorf("invalid caveat validator: expected %d output, not %d", want, got)
	}
	if got, want := fn.Type().Out(0), rtErr; got != want {
		return fmt.Errorf("invalid caveat validator: output must be %v, not %v", want, got)
	}
	if got, want := fn.Type().NumIn(), 2; got != want {
		return fmt.Errorf("invalid caveat validator: expected %d inputs, not %d", want, got)
	}
	if got, want := fn.Type().In(0), rtContext; got != want {
		return fmt.Errorf("invalid caveat validator: first argument must be %v, not %v", want, got)
	}
	if got, want := fn.Type().In(1), param; got != want {
		return fmt.Errorf("invalid caveat validator: second argument must be %v, not %v", want, got)
	}
	r.byUUID[d.Id] = registryEntry{d, fn, param, registerer}
	return nil
}

func (r *caveatRegistry) lookup(uid uniqueid.Id) (registryEntry, bool) {
	r.mu.RLock()
	entry, exists := r.byUUID[uid]
	r.mu.RUnlock()
	return entry, exists
}

func (r *caveatRegistry) validate(uid uniqueid.Id, ctx Context, paramvom []byte) error {
	entry, exists := r.lookup(uid)
	// TODO(ashankar): Figure out a way to get the appropriate language
	// here. Perhaps security.Context should include context.T? Or maybe,
	// check if ctx happens to include it (for example, if ctx is
	// ipc.ServerContext)?
	if !exists {
		return NewErrCaveatNotRegistered(nil, uid)
	}
	param := reflect.New(entry.paramType).Interface()
	if err := vom.Decode(paramvom, param); err != nil {
		t, _ := vdl.TypeFromReflect(entry.paramType)
		return NewErrCaveatParamCoding(nil, uid, t, err.Error())
	}
	err := entry.validatorFn.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(param).Elem()})[0].Interface()
	if err == nil {
		return nil
	}
	return NewErrCaveatValidation(nil, err.(error).Error())
}

// RegisterCaveatValidator associates a CaveatDescriptor with the
// implementation of the validation function.
//
// It may be called at most once per c.ID, and will panic on duplicate
// registrations.
func RegisterCaveatValidator(c CaveatDescriptor, validator interface{}) {
	if err := registry.register(c, validator); err != nil {
		panic(err)
	}
}

// NewCaveat returns a Caveat that requires validation by the validation
// function correponding to c and uses the provided parameters.
func NewCaveat(c CaveatDescriptor, param interface{}) (Caveat, error) {
	got := vdl.TypeOf(param)
	// If the user inputs a vdl.Value, use the type of the vdl.Value instead.
	if vv, ok := param.(*vdl.Value); ok {
		got = vv.Type()
	}
	noAnyInParam := c.ParamType.Walk(vdl.WalkAll, func(t *vdl.Type) bool {
		return t.Kind() != vdl.Any
	})
	if !noAnyInParam {
		return Caveat{}, NewErrCaveatParamAny(nil, c.Id)
	}
	if want := c.ParamType; got != want {
		return Caveat{}, NewErrCaveatParamTypeMismatch(nil, c.Id, got, want)
	}
	bytes, err := vom.Encode(param)
	if err != nil {
		return Caveat{}, NewErrCaveatParamCoding(nil, c.Id, c.ParamType, err.Error())
	}
	return Caveat{c.Id, bytes}, nil
}

// ExpiryCaveat returns a Caveat that validates iff the current time is before t.
func ExpiryCaveat(t time.Time) (Caveat, error) {
	c, err := NewCaveat(UnixTimeExpiryCaveatX, t.Unix())
	if err != nil {
		return c, err
	}
	return c, nil
}

// MethodCaveat returns a Caveat that validates iff the method being invoked by
// the peer is listed in an argument to this function.
func MethodCaveat(method string, additionalMethods ...string) (Caveat, error) {
	c, err := NewCaveat(MethodCaveatX, append(additionalMethods, method))
	if err != nil {
		return c, err
	}
	return c, nil
}

// digest returns a hash of the contents of c.
func (c *Caveat) digest(hash Hash) []byte {
	return hash.sum(append(hash.sum(c.Id[:]), hash.sum(c.ParamVom)...))
}

// Validate tests if c is satisfied under ctx, returning nil if it is or an
// error otherwise.
func (c *Caveat) Validate(ctx Context) error {
	return registry.validate(c.Id, ctx, c.ParamVom)
}

// ThirdPartyDetails returns nil if c is not a third party caveat, or details about
// the third party otherwise.
func (c *Caveat) ThirdPartyDetails() ThirdPartyCaveat {
	if c.Id == PublicKeyThirdPartyCaveatX.Id {
		var param publicKeyThirdPartyCaveat
		if err := vom.Decode(c.ParamVom, &param); err != nil {
			vlog.Errorf("Error decoding PublicKeyThirdPartyCaveat: %v", err)
		}
		return &param
	}
	return nil
}

func (c Caveat) String() string {
	var param interface{}
	if err := vom.Decode(c.ParamVom, &param); err == nil {
		return fmt.Sprintf("%v(%T=%v)", c.Id, param, param)
	}
	return fmt.Sprintf("%v(%d bytes of param)", c.Id, len(c.ParamVom))
}

// UnconstrainedUse returns a Caveat implementation that never fails to
// validate. This is useful only for providing unconstrained
// blessings/discharges to another principal.
func UnconstrainedUse() Caveat {
	return unconstrainedUseCaveat
}

// NewPublicKeyCaveat returns a third-party caveat, i.e., the returned
// Caveat will be valid only when a discharge signed by discharger
// is issued.
//
// Location specifies the expected address at which the third-party
// service is found (and which issues discharges).
//
// The discharger will validate all provided caveats (caveat,
// additionalCaveats) before issuing a discharge.
func NewPublicKeyCaveat(discharger PublicKey, location string, requirements ThirdPartyRequirements, caveat Caveat, additionalCaveats ...Caveat) (Caveat, error) {
	param := publicKeyThirdPartyCaveat{
		Caveats:                append(additionalCaveats, caveat),
		DischargerLocation:     location,
		DischargerRequirements: requirements,
	}
	var err error
	if param.DischargerKey, err = discharger.MarshalBinary(); err != nil {
		return Caveat{}, err
	}
	if _, err := rand.Read(param.Nonce[:]); err != nil {
		return Caveat{}, err
	}
	c, err := NewCaveat(PublicKeyThirdPartyCaveatX, param)
	if err != nil {
		return c, err
	}
	return c, nil
}

func (c *publicKeyThirdPartyCaveat) ID() string {
	key, err := c.discharger()
	if err != nil {
		vlog.Error(err)
		return ""
	}
	hash := key.hash()
	bytes := append(hash.sum(c.Nonce[:]), hash.sum(c.DischargerKey)...)
	for _, cav := range c.Caveats {
		bytes = append(bytes, cav.digest(hash)...)
	}
	return base64.StdEncoding.EncodeToString(hash.sum(bytes))
}

func (c *publicKeyThirdPartyCaveat) Location() string { return c.DischargerLocation }
func (c *publicKeyThirdPartyCaveat) Requirements() ThirdPartyRequirements {
	return c.DischargerRequirements
}

func (c *publicKeyThirdPartyCaveat) Dischargeable(ctx Context) error {
	// Validate the caveats embedded within this third-party caveat.
	for _, cav := range c.Caveats {
		if err := cav.Validate(ctx); err != nil {
			return fmt.Errorf("could not validate embedded restriction(%v): %v", cav, err)
		}
	}
	return nil
}

func (c *publicKeyThirdPartyCaveat) discharger() (PublicKey, error) {
	key, err := UnmarshalPublicKey(c.DischargerKey)
	if err != nil {
		return nil, fmt.Errorf("invalid %T: failed to unmarshal discharger's public key: %v", *c, err)
	}
	return key, nil
}

func (c publicKeyThirdPartyCaveat) String() string {
	return fmt.Sprintf("%v@%v [%+v]", c.ID(), c.Location(), c.Requirements())
}

func (d *publicKeyDischarge) ID() string { return d.ThirdPartyCaveatID }
func (d *publicKeyDischarge) ThirdPartyCaveats() []ThirdPartyCaveat {
	var ret []ThirdPartyCaveat
	for _, cav := range d.Caveats {
		if tp := cav.ThirdPartyDetails(); tp != nil {
			ret = append(ret, tp)
		}
	}
	return ret
}

func (d *publicKeyDischarge) digest(hash Hash) []byte {
	msg := hash.sum([]byte(d.ThirdPartyCaveatID))
	for _, cav := range d.Caveats {
		msg = append(msg, cav.digest(hash)...)
	}
	return hash.sum(msg)
}

func (d *publicKeyDischarge) verify(key PublicKey) error {
	if !bytes.Equal(d.Signature.Purpose, dischargePurpose) {
		return fmt.Errorf("signature on discharge for caveat %v was not intended for discharges(purpose=%q)", d.ThirdPartyCaveatID, d.Signature.Purpose)
	}
	if !d.Signature.Verify(key, d.digest(key.hash())) {
		return fmt.Errorf("signature verification on discharge for caveat %v failed", d.ThirdPartyCaveatID)
	}
	return nil
}

func (d *publicKeyDischarge) sign(signer Signer) error {
	var err error
	d.Signature, err = signer.Sign(dischargePurpose, d.digest(signer.PublicKey().hash()))
	return err
}
func (d *publicKeyDischarge) toWire() WireDischarge { return WireDischargePublicKey{*d} }
