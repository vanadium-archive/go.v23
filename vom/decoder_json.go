package vom

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

// decoderJSON decodes type and value messages from the JSON format.
type decoderJSON struct {
	s     scannerJSON
	state decState
	// For managing type definitions
	wireStrs map[string]string
	wireDefs *nameResolver
}

func newDecoderJSON(decbuf *decbuf) *decoderJSON {
	return &decoderJSON{
		s:        makeScannerJSON(decbuf),
		wireStrs: make(map[string]string),
		wireDefs: newNameResolver(),
	}
}

func (d *decoderJSON) SetMaxStringLength(max int) {
	d.s.SetMaxStringLength(max)
}

// DecodeMsg decodes a single JSON message from decbuf, and returns true if it
// is a value message (rather than a type message).
func (d *decoderJSON) DecodeMsg(rv Value) (bool, error) {
	// We've already read past the the initial '[' when this function is called.
	d.s.Reset(jsonStateArray)
	defer d.s.SkipToEnd()
	d.state.reset()
	switch tok := d.s.ReadToken(); tok {
	case jsonEndArray:
		// Silently skip empty arrays.
		return false, nil
	case jsonStartString:
		label, err := d.s.ReadString()
		if err != nil {
			return false, err
		}
		if label == "type" {
			return false, d.decodeTypeMsg()
		}
		return true, d.decodeValueMsg(label, rv)
	default:
		return false, d.s.TokenErr(tok, "vom: json msg doesn't start with label")
	}
}

func (d *decoderJSON) decodeTypeMsg() error {
	// Inside array, next item must be string typedef.
	if tok := d.s.ReadToken(); tok != jsonCommaSym {
		return d.s.TokenErr(tok, "vom: json type msg missing typedef comma")
	}
	if tok := d.s.ReadToken(); tok != jsonStartString {
		return d.s.TokenErr(tok, "vom: json type msg missing typedef string")
	}
	typedef, err := d.s.ReadString()
	if err != nil {
		return err
	}
	// Split the typedef into name and def.
	name, defstr, err := splitNameDef(typedef, namePunctOK)
	if err != nil {
		return err
	}
	var tags []string
	tok := d.s.ReadToken()
	if tok == jsonCommaSym {
		// Parse optional tags.
		var err error
		if tags, err = d.decodeTypeTags(); err != nil {
			return err
		}
		tok = d.s.ReadToken()
	}
	if tok != jsonEndArray {
		return d.s.TokenErr(tok, "vom: json expected end type msg")
	}
	// Add entries to wireDefs and wireStrs.  The strategy: allow the name to be
	// immediately resolvable under its short name when we process the type msg,
	// but postpone actually defining the type until it's first used in a value
	// msg.  We can't define the type now since we might not have seen all types
	// for recursive types.  But making the name resolvable lets the user use
	// short names in type definitions.
	if d.wireDefs.resolveName(name) != nil {
		return fmt.Errorf("vom: json duplicate typedef name %q", name)
	}
	def := &wireDef{name: name, tags: tags}
	def.startDefine()
	d.wireDefs.insert(def)
	d.wireDefs.enableShortNames(def)
	d.wireStrs[name] = defstr
	return nil
}

type namePunctMode bool

const (
	namePunctOK         namePunctMode = true
	namePunctDisallowed namePunctMode = false
	whitespace                        = " \n\r\t"
)

// splitNameDef splits namedef into its name and defstr.  The mode determines
// whether extra punctuation is allowed in the name - e.g. typenames allow '.'
// and '/' while struct field names don't.
func splitNameDef(namedef string, mode namePunctMode) (string, string, error) { // name, defstr, err
	spaceIndex := strings.IndexAny(namedef, whitespace)
	if spaceIndex == -1 {
		return "", "", fmt.Errorf("vom: json missing whitespace between name and def: %q", namedef)
	}
	name := namedef[:spaceIndex]
	defstr := strings.Trim(namedef[spaceIndex+1:], whitespace)
	if name == "" {
		return "", "", fmt.Errorf("vom: json empty type name: %q", namedef)
	}
	// Validate the type name - we have the same requirements as Go.
	first := true
	for _, r := range name {
		var good bool
		if first {
			good = unicode.IsLetter(r) || r == '_'
		} else if mode == namePunctDisallowed {
			good = unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
		} else {
			good = unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == '/'
		}
		if !good {
			return "", "", fmt.Errorf("vom: json name has invalid runes: %q", name)
		}
		first = false
	}
	return name, defstr, nil
}

func (d *decoderJSON) decodeTypeTags() ([]string, error) {
	var result []string
	rv := ValueOf(&result).Elem()
	rti, _ := lookupRTInfo(rv.Type())
	if err := d.decodeTypedValue(wdStringSlice, rti, rv); err != nil {
		return nil, err
	}
	return result, nil
}

func (d *decoderJSON) decodeValueMsg(typename string, rv Value) error {
	def, err := d.lookupWireDef(typename)
	if err != nil {
		return err
	}
	if tok := d.s.ReadToken(); tok != jsonCommaSym {
		return d.s.TokenErr(tok, "vom: json typed value %q missing value", def.name)
	}
	if rv.IsValid() {
		rti, err := lookupRTInfo(rv.Type())
		if err != nil {
			return err
		}
		if err := d.decodeTypedValue(def, rti, rv); err != nil {
			return err
		}
	} else {
		if err := d.ignoreTypedValue(def); err != nil {
			return err
		}
	}
	if tok := d.s.ReadToken(); tok != jsonEndArray {
		return d.s.TokenErr(tok, "vom: json expected end value msg")
	}
	return nil
}

// lookupWireDef returns the wireDef given the typename.
func (d *decoderJSON) lookupWireDef(typename string) (*wireDef, error) {
	if typename == "" {
		return nil, fmt.Errorf("vom: json empty type name")
	}
	// Bootstrap wire definitions always exist.
	if def, ok := bootstrapWireDefsFromName[typename]; ok {
		return def, nil
	}
	// Lookup in wireDefs.  This acts as a cache of types that have already been
	// defined, and also serves to terminate lookups for recursive types.
	if idef := d.wireDefs.resolveName(typename); idef != nil {
		def := idef.(*wireDef)
		if def.defined {
			return def, nil
		}
		// The type hasn't been defined yet.  Define it now, keeping the original
		// def pointer.  Infinite loops from recursive types are broken by setting
		// def.defined here, before recursing.
		def.defined = true
		fullname := def.name
		snames := def.snames
		tags := def.tags
		defstr, ok := d.wireStrs[fullname]
		if !ok {
			return nil, fmt.Errorf("vom: json internal type missing: %q", fullname)
		}
		base, err := d.lookupWireDef(defstr)
		if err != nil {
			return nil, err
		}
		delete(d.wireStrs, fullname)
		*def = *base
		def.name = fullname
		def.snames = snames
		def.tags = tags
		def.finishDefine()
		return def, nil
	}
	// None of the lookups worked - try to make an unnamed type.
	def, err := d.makeUnnamedType(typename)
	if err != nil {
		return nil, err
	}
	def.define()
	// Check wireDefs again.  This is necessary to avoid inserting recursive types
	// multiple times, since makeUnnamedType could have inserted the type first.
	//
	// Unnamed types always appear in wireDefs under their full name; if the user
	// defines "veyron/lib/vom.Foo" and refers to a pointer as "*Foo", wireDefs
	// will only contain "*veyron/lib/vom.Foo".  Subsequent lookups of "*Foo" will
	// make the unnamed type again, and discover it's already defined here.
	// TODO(toddw): Insert the short name "*Foo" as well, and remove it when there
	// is a shortname collision.
	if idef := d.wireDefs.resolveName(def.name); idef != nil {
		return idef.(*wireDef), nil
	}
	d.wireDefs.insert(def)
	return def, nil
}

// makeUnnamedType makes the wireDef for the given typedesc.
//
// TODO(toddw): Allow arbitrary whitespace in type definitions.
func (d *decoderJSON) makeUnnamedType(typedesc string) (*wireDef, error) {
	// Look for pointer.
	if strings.HasPrefix(typedesc, "*") {
		elem, err := d.lookupWireDef(typedesc[1:])
		if err != nil {
			return nil, err
		}
		return &wireDef{kind: typeKindPtr, elem: elem, numStars: 1}, nil
	}
	// Look for slice.
	if strings.HasPrefix(typedesc, "[]") {
		elem, err := d.lookupWireDef(typedesc[2:])
		if err != nil {
			return nil, err
		}
		return &wireDef{kind: typeKindSlice, elem: elem}, nil
	}
	// Look for array.
	if strings.HasPrefix(typedesc, "[") {
		lenstr, elemstr, err := splitTypeEndingWith(typedesc[1:], ']')
		if err != nil {
			return nil, err
		}
		len, err := strconv.Atoi(lenstr)
		if err != nil {
			return nil, err
		}
		elem, err := d.lookupWireDef(elemstr)
		if err != nil {
			return nil, err
		}
		return &wireDef{kind: typeKindArray, len: len, elem: elem}, nil
	}
	// Look for map.
	if strings.HasPrefix(typedesc, "map[") {
		keystr, elemstr, err := splitTypeEndingWith(typedesc[4:], ']')
		if err != nil {
			return nil, err
		}
		key, err := d.lookupWireDef(keystr)
		if err != nil {
			return nil, err
		}
		elem, err := d.lookupWireDef(elemstr)
		if err != nil {
			return nil, err
		}
		return &wireDef{kind: typeKindMap, key: key, elem: elem}, nil
	}
	// Look for struct.
	if strings.HasPrefix(typedesc, "struct{") {
		if typedesc[len(typedesc)-1] != '}' {
			return nil, fmt.Errorf("vom: json struct unmatched '}': %q", typedesc)
		}
		if len(typedesc) == 8 {
			// Special-case for empty struct.
			return &wireDef{kind: typeKindStruct}, nil
		}
		fields := strings.Split(typedesc[7:len(typedesc)-1], ";")
		def := &wireDef{kind: typeKindStruct, fields: make([]wireDefField, len(fields))}
		for fx, field := range fields {
			name, fstr, err := splitNameDef(field, namePunctDisallowed)
			if err != nil {
				return nil, err
			}
			fdef, err := d.lookupWireDef(fstr)
			if err != nil {
				return nil, err
			}
			def.fields[fx].name = name
			def.fields[fx].def = fdef
		}
		return def, nil
	}
	return nil, fmt.Errorf("vom: json couldn't make type %q", typedesc)
}

// splitTypeEndingWith is a scanning routine to help break down a type
// description into individual pieces.  The strategy is to simply count
// occurrences of brackets and curly braces; it's only a scanner so it doesn't
// matter if it splits invalid type descriptions incorrectly, it just needs to
// split valid type descriptions correctly.
//
// Example:
//  name="map[struct{A []byte}]string"
//  splitTypeEndingWith(name[4, ']') = ("struct{A []byte}", "string", nil)
func splitTypeEndingWith(name string, end rune) (string, string, error) {
	var numArray, numObject int
	for rx, r := range name {
		switch {
		case r == '[':
			numArray++
		case r == '{':
			numObject++
		case r == end && numArray == 0 && numObject == 0:
			return name[:rx], name[rx+1:], nil
		case r == ']':
			if numArray == 0 {
				return "", "", fmt.Errorf("vom: json type unmatched ']': %q", name)
			}
			numArray--
		case r == '}':
			if numObject == 0 {
				return "", "", fmt.Errorf("vom: json type unmatched '}': %q", name)
			}
			numObject--
		default:
			// Skip forward past all other runes.
		}
	}
	return "", "", fmt.Errorf("vom: json type no ending %q: %q", end, name)
}

// decodeTypedValue is identical to the same method in decoderBinary - it calls
// decodeTypedValueHelper for actual per-kind decoding.  Trying to share the
// code incurs a 2x slowdown.  Keep the two versions in sync.
func (d *decoderJSON) decodeTypedValue(def *wireDef, rti *rtInfo, rv Value) error {
	// Here's the pointer handling strategy.  If def and rv have the same
	// structure (i.e. same number of pointers), we want pointer sharing and nils
	// to be preserved in the decoded result.  If rv has more stars than def, we
	// should be able to dereference the decoded result until they have the same
	// number of stars, and again the pointer sharing and nils should be
	// preserved.
	//
	// We use alignStars to make this happen.  If rv is a pointer type and has
	// more stars than def, alignStars will dereference until we end up with the
	// same number of stars.  Thereafter calls to decodePtr will walk def and rv
	// in lockstep, re-creating the same sharing structure.
	//
	// There is a catch wrt interfaces.  Let's say the encoded value represented
	// by def is ***interface{}, where the interface holds concrete value **int.
	// Intuitively we'd expect to be able to decode into a concrete *****int and
	// get the proper sharing.  However this would be complicated to implement,
	// since we don't know the type of the concrete value held in the interface
	// until we actually decode it.  So we punt on this unusual case.
	rti, rv = alignStars(def, rti, rv, FormatBinary)

	if rti.kind != typeKindInterface || def.kind == typeKindInterface {
		// Either the item we're decoding into (rti) isn't an interface, or both the
		// item we're decoding into and the encoded value (def) are interfaces, so
		// pass through to the helper directly.  For the latter case we let the
		// decodeInterface logic run; if the encoded value is nil, rv will also be
		// set to nil, which is the zero value.  Otherwise decodeTypedValue will be
		// recursively called again, but with the concrete def.
		return d.decodeTypedValueHelper(def, rti, rv)
	}

	// The item we're decoding into is an interface, and the encoded value is
	// concrete.  We must fill in rvIface with a value and call rv.Set afterwards,
	// since interfaces have value semantics and can't be modified by side-effect.
	var rvIface Value
	switch {
	case !rv.IsNil():
		// Create a new instance of the underlying concrete value to fill in.
		rvIface = New(rv.Elem().Type()).Elem()
	case def.rt != nil:
		// We're performing generic decoding into a nil interface - create the
		// appropriate type of value based on the encoded value (def).  Note that
		// rv may be a nil interface with actual methods defined, so we need to
		// check assignability.
		if rti.numMethods > 0 && !def.rt.AssignableTo(rv.Type()) {
			return fmt.Errorf("vom: can't assign type %q to value of type %q", def.rt, rv.Type())
		}
		rvIface = New(def.rt).Elem()
	default:
		return fmt.Errorf("vom: type not registered or creatable, type %q", def)
	}
	rtiIface, err := lookupRTInfo(rvIface.Type())
	if err != nil {
		return err
	}
	if err := d.decodeTypedValueHelper(def, rtiIface, rvIface); err != nil {
		return err
	}
	return rv.trySet(rvIface)
}

func (d *decoderJSON) decodeTypedValueHelper(def *wireDef, rti *rtInfo, rv Value) error {
	var customDecode *customDecode
	rti, rv, customDecode = rti.initCustomDecode(rv, FormatJSON)

	// We need to align the pointers again since the argument to the VomDecode
	// method may be a pointer type.
	rti, rv = alignStars(def, rti, rv, FormatJSON)

	// Decode the encoded value into rv.
	var err error
	switch def.kind {
	case typeKindBool:
		err = d.decodeBool(rti, rv)
	case typeKindString:
		err = d.decodeString(rti, rv)
	case typeKindByteSlice:
		err = d.decodeByteSlice(rti, rv)
	case typeKindByte, typeKindUint, typeKindInt, typeKindFloat:
		err = d.decodeNumber(def, rti, rv)
	case typeKindSlice, typeKindArray:
		if def.elem.kind == typeKindByte {
			err = d.decodeByteSlice(rti, rv)
		} else {
			err = d.decodeArray(def, rti, rv)
		}
	case typeKindMap:
		err = d.decodeMap(def, rti, rv)
	case typeKindStruct:
		err = d.decodeStruct(def, rti, rv)
	case typeKindPtr:
		err = d.decodePtr(def, rti, rv)
	case typeKindInterface:
		err = d.decodeInterface(rti, rv)
	default:
		panic(fmt.Errorf("vom: json decodeTypedValueHelper unhandled %v %+v", def.kind, def))
	}
	if err != nil {
		return err
	}
	// Call the custom decoder method if it exists, otherwise we've already
	// decoded into the final value by side-effect.
	if customDecode != nil {
		return customDecode.call()
	}
	return nil
}

func (d *decoderJSON) decodeBool(rti *rtInfo, rv Value) error {
	switch tok := d.s.ReadToken(); tok {
	case jsonTrue:
		return fillFromBool(true, rti, rv)
	case jsonFalse:
		return fillFromBool(false, rti, rv)
	default:
		return d.s.TokenErr(tok, "vom: json expected bool")
	}
}

func (d *decoderJSON) decodeString(rti *rtInfo, rv Value) error {
	if tok := d.s.ReadToken(); tok != jsonStartString {
		return d.s.TokenErr(tok, "vom: json expected string")
	}
	bs, isNewSlice, err := d.s.ReadStringAsSlice()
	if err != nil {
		return err
	}
	return fillFromByteSlice(bs, isNewSlice, rti, rv)
}

func (d *decoderJSON) decodeByteSlice(rti *rtInfo, rv Value) error {
	if tok := d.s.ReadToken(); tok != jsonStartString {
		return d.s.TokenErr(tok, "vom: json expected byte slice")
	}
	str, err := d.s.ReadString()
	if err != nil {
		return err
	}
	bs, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return err
	}
	return fillFromByteSlice(bs, true, rti, rv)
}

func (d *decoderJSON) decodeNumber(def *wireDef, rti *rtInfo, rv Value) error {
	var numbytes []byte
	var err error
	switch tok := d.s.ReadToken(); tok {
	case jsonStartNumber:
		numbytes, err = d.s.ScanNumber()
	case jsonStartString:
		numbytes, _, err = d.s.ReadStringAsSlice()
	default:
		return d.s.TokenErr(tok, "vom: json expected number")
	}
	if err != nil {
		return err
	}
	switch def.kind {
	case typeKindByte, typeKindUint:
		uval, err := parseUint(numbytes)
		if err != nil {
			return err
		}
		return fillFromUint(uval, rti, rv)
	case typeKindInt:
		ival, err := parseInt(numbytes)
		if err != nil {
			return err
		}
		return fillFromInt(ival, rti, rv)
	case typeKindFloat:
		fval, err := strconv.ParseFloat(string(numbytes), floatBitSize(def.id))
		if err != nil {
			return err
		}
		return fillFromFloat(fval, rti, rv)
	default:
		panic(fmt.Errorf("vom: json decodeNumber unhandled kind %v", def.kind))
	}
}

var (
	errNumberSyntax   = errors.New("vom: invalid number syntax")
	errNumberOverflow = errors.New("vom: number overflow")
)

func parseUint(bs []byte) (uint64, error) {
	bslen := len(bs)
	if bslen == 0 {
		return 0, errNumberSyntax
	}
	// Disallow leading 0 for multi-digit number, as per JSON spec.
	b := bs[0]
	if b < '0' || b > '9' || (b == '0' && bslen > 1) {
		return 0, errNumberSyntax
	}
	// Loop through the rest of the digits, checking for overflow.
	uval := uint64(b - '0')
	for ix := 1; ix < bslen; ix++ {
		b := bs[ix]
		if b < '0' || b > '9' {
			return 0, errNumberSyntax
		}
		tmp := uval
		tmp *= 10
		tmp += uint64(b - '0')
		if tmp < uval {
			return 0, errNumberOverflow
		}
		uval = tmp
	}
	return uval, nil
}

func parseInt(bs []byte) (int64, error) {
	if len(bs) == 0 {
		return 0, errNumberSyntax
	}
	// Pick off leading '-', if it exists.
	neg := false
	if bs[0] == '-' {
		bs = bs[1:]
		neg = true
	}
	// Parse the rest as a uint.
	uval, err := parseUint(bs)
	if err != nil {
		return 0, err
	}
	// Check for overflow and return result.
	if neg {
		if uval > -math.MinInt64 {
			return 0, errNumberOverflow
		}
		return -int64(uval), nil
	}
	if uval > math.MaxInt64 {
		return 0, errNumberOverflow
	}
	return int64(uval), nil
}

func (d *decoderJSON) decodeArray(def *wireDef, rti *rtInfo, rv Value) error {
	isSlice := rti.rt.Kind() == reflect.Slice
	if !isSlice && rti.rt.Kind() != reflect.Array {
		return fmt.Errorf("vom: json type mismatch, can't decode array %q into value of type %q", def, rti.rt)
	}
	switch tok := d.s.ReadToken(); {
	case tok == jsonNull:
		if isSlice {
			rv.SetLen(0)
		}
		return nil
	case tok != jsonStartArray:
		return d.s.TokenErr(tok, "vom: json expected array")
	}
	if d.s.ReadTokenOnlyIf(jsonEndArray) {
		if isSlice {
			rv.SetLen(0)
		}
		return nil
	}
	if isSlice && rv.Len() < rv.Cap() {
		rv.SetLen(rv.Cap())
	}
	ix := 0
	for {
		if ix >= rv.Len() {
			if !isSlice {
				return fmt.Errorf("vom: json array has more items than array %q", rti.name)
			}
			// If we run out of space, double the size of the slice.
			const minlen = 10
			newlen := rv.Len() * 2
			if newlen < minlen {
				newlen = minlen
			}
			rvNew := MakeSlice(rti.rt, newlen, newlen)
			if err := Copy(rvNew, rv); err != nil {
				return err
			}
			rv.Set(rvNew)
		}
		if err := d.decodeTypedValue(def.elem, rti.elem, rv.Index(ix)); err != nil {
			return err
		}
		switch tok := d.s.ReadToken(); {
		case tok == jsonEndArray:
			if isSlice {
				rv.SetLen(ix + 1)
			}
			return nil
		case tok != jsonCommaSym:
			return d.s.TokenErr(tok, "vom: json expected comma for next array item")
		}
		// Move on to the next item.
		ix++
	}
}

func (d *decoderJSON) decodeMap(def *wireDef, rti *rtInfo, rv Value) error {
	if rti.rt.Kind() != reflect.Map {
		return fmt.Errorf("vom: json type mismatch, can't decode map %q into value of type %q", def, rti.rt)
	}
	switch tok := d.s.ReadToken(); {
	case tok == jsonNull:
		if rv.Len() != 0 {
			rv.Set(rti.zero)
		}
		return nil
	case tok != jsonStartObject:
		return d.s.TokenErr(tok, "vom: json expected map object")
	}
	if d.s.ReadTokenOnlyIf(jsonEndObject) {
		if rv.Len() != 0 {
			rv.Set(rti.zero)
		}
		return nil
	}
	rv.Set(MakeMap(rti.rt))
	key := New(rti.key.rt).Elem()
	elem := New(rti.elem.rt).Elem()
	// JSON map keys must be strings, and are represented by encoding the key as
	// usual and adding quotes and escaping.
	isKeyQuoted := def.key.kind != typeKindString && def.key.kind != typeKindByteSlice
	for {
		if isKeyQuoted {
			if tok := d.s.ReadToken(); tok != jsonStartString {
				return d.s.TokenErr(tok, "vom: json expected quote for map key")
			}
		}
		key.Set(rti.key.zero)
		if err := d.decodeTypedValue(def.key, rti.key, key); err != nil {
			return err
		}
		if isKeyQuoted {
			if err := d.s.ReadEndString(); err != nil {
				return err
			}
		}
		if tok := d.s.ReadToken(); tok != jsonColonSym {
			return d.s.TokenErr(tok, "vom: json expected colon for map pair")
		}
		elem.Set(rti.elem.zero)
		if err := d.decodeTypedValue(def.elem, rti.elem, elem); err != nil {
			return err
		}
		rv.SetMapIndex(key, elem)
		switch tok := d.s.ReadToken(); {
		case tok == jsonEndObject:
			return nil
		case tok != jsonCommaSym:
			return d.s.TokenErr(tok, "vom: json expected comma for next map item")
		}
	}
}

func (d *decoderJSON) decodeStruct(def *wireDef, rti *rtInfo, rv Value) error {
	if rti.rt.Kind() != reflect.Struct {
		return fmt.Errorf("vom: json type mismatch, can't decode struct %q into value of type %q", def, rti.rt)
	}
	if tok := d.s.ReadToken(); tok != jsonStartObject {
		return d.s.TokenErr(tok, "vom: json expected struct object")
	}
	if d.s.ReadTokenOnlyIf(jsonEndObject) {
		return nil
	}
	// TODO(toddw): Throw an error if the encoded and decoded structs have no
	// fields in common.  We should only perform that check once per encode /
	// decode type pair - it'll be easier to add when we optimize the code.
	for {
		if tok := d.s.ReadToken(); tok != jsonStartString {
			return d.s.TokenErr(tok, "vom: json expected quote for struct field name")
		}
		fname, err := d.s.ReadString()
		if err != nil {
			return err
		}
		if tok := d.s.ReadToken(); tok != jsonColonSym {
			return d.s.TokenErr(tok, "vom: json expected colon for struct field")
		}
		fdef := def.fieldByName(fname)
		if fdef == nil {
			return fmt.Errorf("vom: json struct %q doesn't have field %q", def, fname)
		}
		if findex, finfo := rti.fieldByName(fname); findex != -1 {
			// Decode into the matching field name.
			err = d.decodeTypedValue(fdef, finfo, rv.Field(findex))
		} else {
			err = d.ignoreTypedValue(fdef)
		}
		if err != nil {
			return err
		}
		switch tok := d.s.ReadToken(); {
		case tok == jsonEndObject:
			return nil
		case tok != jsonCommaSym:
			return d.s.TokenErr(tok, "vom: json expected comma for next struct item")
		}
	}
}

func (d *decoderJSON) decodePtr(def *wireDef, rti *rtInfo, rv Value) error {
	switch tok := d.s.ReadToken(); {
	case tok == jsonNull:
		// TODO(toddw): Check compatibility between def and rti.
		rv.Set(rti.zero)
		return nil
	case tok != jsonStartArray:
		return d.s.TokenErr(tok, "vom: json expected start array for ptr")
	}
	var refID int64
	rvRefID := ValueOf(&refID).Elem()
	if err := d.decodeNumber(wdInt64, rtInfoInt64, rvRefID); err != nil {
		return err
	}
	if refID == 0 {
		// TODO(toddw): Check compatibility between def and rti.
		rv.Set(rti.zero)
		return nil
	}
	switch tok := d.s.ReadToken(); {
	case tok == jsonEndArray:
		// Solo refID represents a reference to a previously encountered value.
		return d.state.fillFromRefID(refID, rti, rv)
	case tok != jsonCommaSym:
		return d.s.TokenErr(tok, "vom: json expected comma for ptr value")
	}
	// We're defining a new refID along with its associated value.
	var err error
	if rti, rv, err = d.state.defineRefID(refID, rti, rv); err != nil {
		return err
	}
	if err := d.decodeTypedValue(def.elem, rti, rv); err != nil {
		return err
	}
	if tok := d.s.ReadToken(); tok != jsonEndArray {
		return d.s.TokenErr(tok, "vom: json expected end array for ptr")
	}
	return nil
}

func (d *decoderJSON) decodeInterface(rti *rtInfo, rv Value) error {
	switch tok := d.s.ReadToken(); {
	case tok == jsonNull:
		rv.Set(rti.zero)
		return nil
	case tok != jsonStartArray:
		return d.s.TokenErr(tok, "vom: json expected start array for interface")
	}
	switch tok := d.s.ReadToken(); {
	case tok == jsonEndArray:
		rv.Set(rti.zero)
		return nil
	case tok != jsonStartString:
		return d.s.TokenErr(tok, "vom: json expected quote for interface typename")
	}
	typename, err := d.s.ReadString()
	if err != nil {
		return err
	}
	if tok := d.s.ReadToken(); tok != jsonCommaSym {
		return d.s.TokenErr(tok, "vom: json expected comma for interface value")
	}
	def, err := d.lookupWireDef(typename)
	if err != nil {
		return err
	}
	if err := d.decodeTypedValue(def, rti, rv); err != nil {
		return err
	}
	if tok := d.s.ReadToken(); tok != jsonEndArray {
		return d.s.TokenErr(tok, "vom: json expected end array for interface")
	}
	return nil
}

// The ignore* functions are just like the decode* functions, except they ignore
// the encoded value, and thus we don't need a Value to decode into.

func (d *decoderJSON) ignoreTypedValue(def *wireDef) error {
	switch def.kind {
	case typeKindBool:
		return d.ignoreBool()
	case typeKindString:
		return d.ignoreString()
	case typeKindByteSlice:
		return d.ignoreByteSlice()
	case typeKindByte, typeKindUint, typeKindInt, typeKindFloat:
		return d.ignoreNumber(def)
	case typeKindSlice, typeKindArray:
		if def.elem.kind == typeKindByte {
			return d.ignoreByteSlice()
		}
		return d.ignoreArray(def)
	case typeKindMap:
		return d.ignoreMap(def)
	case typeKindStruct:
		return d.ignoreStruct(def)
	case typeKindPtr:
		return d.ignorePtr(def)
	case typeKindInterface:
		return d.ignoreInterface()
	default:
		panic(fmt.Errorf("vom: json ignoreTypedValue unhandled %v %+v", def.kind, def))
	}
}

func (d *decoderJSON) ignoreBool() error {
	if tok := d.s.ReadToken(); tok != jsonTrue && tok != jsonFalse {
		return d.s.TokenErr(tok, "vom: json expected bool")
	}
	return nil
}

func (d *decoderJSON) ignoreString() error {
	if tok := d.s.ReadToken(); tok != jsonStartString {
		return d.s.TokenErr(tok, "vom: json expected string")
	}
	_, _, err := d.s.ReadStringAsSlice()
	return err
}

func (d *decoderJSON) ignoreByteSlice() error {
	if tok := d.s.ReadToken(); tok != jsonStartString {
		return d.s.TokenErr(tok, "vom: json expected byte slice")
	}
	str, err := d.s.ReadString()
	if err != nil {
		return err
	}
	_, err = base64.StdEncoding.DecodeString(str)
	return err
}

func (d *decoderJSON) ignoreNumber(def *wireDef) error {
	var numbytes []byte
	var err error
	switch tok := d.s.ReadToken(); tok {
	case jsonStartNumber:
		numbytes, err = d.s.ScanNumber()
	case jsonStartString:
		numbytes, _, err = d.s.ReadStringAsSlice()
	default:
		return d.s.TokenErr(tok, "vom: json expected number")
	}
	if err != nil {
		return err
	}
	switch def.kind {
	case typeKindByte, typeKindUint:
		_, err = parseUint(numbytes)
	case typeKindInt:
		_, err = parseInt(numbytes)
	case typeKindFloat:
		_, err = strconv.ParseFloat(string(numbytes), floatBitSize(def.id))
	default:
		panic(fmt.Errorf("vom: json decodeNumber unhandled kind %v", def.kind))
	}
	return err
}

func (d *decoderJSON) ignoreArray(def *wireDef) error {
	switch tok := d.s.ReadToken(); {
	case tok == jsonNull:
		return nil
	case tok != jsonStartArray:
		return d.s.TokenErr(tok, "vom: json expected array")
	}
	if d.s.ReadTokenOnlyIf(jsonEndArray) {
		return nil
	}
	for {
		if err := d.ignoreTypedValue(def.elem); err != nil {
			return err
		}
		switch tok := d.s.ReadToken(); {
		case tok == jsonEndArray:
			return nil
		case tok != jsonCommaSym:
			return d.s.TokenErr(tok, "vom: json expected comma for next array item")
		}
	}
}

func (d *decoderJSON) ignoreMap(def *wireDef) error {
	switch tok := d.s.ReadToken(); {
	case tok == jsonNull:
		return nil
	case tok != jsonStartObject:
		return d.s.TokenErr(tok, "vom: json expected map object")
	}
	if d.s.ReadTokenOnlyIf(jsonEndObject) {
		return nil
	}
	// JSON map keys must be strings, and are represented by encoding the key as
	// usual and adding quotes and escaping.
	isKeyQuoted := def.key.kind != typeKindString && def.key.kind != typeKindByteSlice
	for {
		if isKeyQuoted {
			if tok := d.s.ReadToken(); tok != jsonStartString {
				return d.s.TokenErr(tok, "vom: json expected quote for map key")
			}
		}
		if err := d.ignoreTypedValue(def.key); err != nil {
			return err
		}
		if isKeyQuoted {
			if err := d.s.ReadEndString(); err != nil {
				return err
			}
		}
		if tok := d.s.ReadToken(); tok != jsonColonSym {
			return d.s.TokenErr(tok, "vom: json expected colon for map pair")
		}
		if err := d.ignoreTypedValue(def.elem); err != nil {
			return err
		}
		switch tok := d.s.ReadToken(); {
		case tok == jsonEndObject:
			return nil
		case tok != jsonCommaSym:
			return d.s.TokenErr(tok, "vom: json expected comma for next map item")
		}
	}
}

func (d *decoderJSON) ignoreStruct(def *wireDef) error {
	if tok := d.s.ReadToken(); tok != jsonStartObject {
		return d.s.TokenErr(tok, "vom: json expected struct object")
	}
	if d.s.ReadTokenOnlyIf(jsonEndObject) {
		return nil
	}
	for {
		if tok := d.s.ReadToken(); tok != jsonStartString {
			return d.s.TokenErr(tok, "vom: json expected quote for struct field name")
		}
		fname, err := d.s.ReadString()
		if err != nil {
			return err
		}
		if tok := d.s.ReadToken(); tok != jsonColonSym {
			return d.s.TokenErr(tok, "vom: json expected colon for struct field")
		}
		fdef := def.fieldByName(fname)
		if fdef == nil {
			return fmt.Errorf("vom: json struct %q doesn't have field %q", def, fname)
		}
		if err := d.ignoreTypedValue(fdef); err != nil {
			return err
		}
		switch tok := d.s.ReadToken(); {
		case tok == jsonEndObject:
			return nil
		case tok != jsonCommaSym:
			return d.s.TokenErr(tok, "vom: json expected comma for next struct item")
		}
	}
}

func (d *decoderJSON) ignorePtr(def *wireDef) error {
	switch tok := d.s.ReadToken(); {
	case tok == jsonNull:
		return nil
	case tok != jsonStartArray:
		return d.s.TokenErr(tok, "vom: json expected start array for ptr")
	}
	if err := d.ignoreNumber(wdInt64); err != nil {
		return err
	}
	switch tok := d.s.ReadToken(); {
	case tok == jsonEndArray:
		return nil
	case tok != jsonCommaSym:
		return d.s.TokenErr(tok, "vom: json expected comma for ptr value")
	}
	if err := d.ignoreTypedValue(def.elem); err != nil {
		return err
	}
	if tok := d.s.ReadToken(); tok != jsonEndArray {
		return d.s.TokenErr(tok, "vom: json expected end array for ptr")
	}
	return nil
}

func (d *decoderJSON) ignoreInterface() error {
	switch tok := d.s.ReadToken(); {
	case tok == jsonNull:
		return nil
	case tok != jsonStartArray:
		return d.s.TokenErr(tok, "vom: json expected start array for interface")
	}
	switch tok := d.s.ReadToken(); {
	case tok == jsonEndArray:
		return nil
	case tok != jsonStartString:
		return d.s.TokenErr(tok, "vom: json expected quote for interface typename")
	}
	typename, err := d.s.ReadString()
	if err != nil {
		return err
	}
	if tok := d.s.ReadToken(); tok != jsonCommaSym {
		return d.s.TokenErr(tok, "vom: json expected comma for interface value")
	}
	def, err := d.lookupWireDef(typename)
	if err != nil {
		return err
	}
	if err := d.ignoreTypedValue(def); err != nil {
		return err
	}
	if tok := d.s.ReadToken(); tok != jsonEndArray {
		return d.s.TokenErr(tok, "vom: json expected end array for interface")
	}
	return nil
}
