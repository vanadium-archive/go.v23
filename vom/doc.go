/*
Package vom implements Veyron Object Marshaling, a serialization protocol
similar to the encoding/gob package.  Vom is used in Veyron to enable
interchange of user-defined data structures across networks, languages and
storage systems.

The vom core API is almost identical to encoding/gob.  To marshal objects create
an Encoder and present it with a series of values.  To unmarshal objects create
a Decoder and retrieve values.  The implementation creates a stream of messages
between the Encoder and Decoder, where each message either contains a type
definition, or encodes a typed value.  The protocol is self-describing; all user
types are defined in terms of a small set of bootstrap primitive and composite
types.

Vom supports encoding and decoding all Go values other than unsafe.Pointer,
functions and channels.  Pointers are translated to self-contained local
references in the wire representation, and captures sharing - if a DAG or graph
is encoded, the structure of the data will be recreated in the decoded value.

Values stored in interfaces are also supported.  The "static" type definition
indicates it is an interface, while the value stores a reference to the
"dynamic" concrete type definition, along with the concrete value.  You may
decode into either an interface or concrete value regardless of whether the
encoded value is an interface or concrete value.  Generic decoding is possible
by decoding into a nil interface value, where the value is automatically created
for you, with type based on the encoded type name.

The types of the encoded and decoded values need not be identical, they only
need to be compatible.  Here are some compatibility rules:

* Numeric types (ints, uints and floats) are compatible if the decoded type can
represent the encoded value without loss; e.g. an encoded float64(255.0) may be
decoded into a value of type uint8, but float64(256.0) or float64(1.1) may not.

* The string and []byte types are compatible.

* Composite types are generally compatible if their contained types are
compatible, subject to the other rules.  E.g. map[float64]string is compatible
with map[uint8][]byte as long as the encoded float64 value is in the range
[0,255].

* Pointers are optional, the "number of stars" need not match.  E.g. an encoded
**float64(255.0) may be decoded into a value of type uint8.  Exception: if the
encoded value represents sharing via pointers, the decoded value must represent
the shared values with a single compatible type.  E.g. if struct{A, B *int8} is
encoded with the same pointer value for A and B, it may be decoded into
struct{A, B float32} or struct{A, B **int32}, but may not be decoded into
struct{A *int, B float64}.

* Struct compatibility is based on their exported fields, identified by name.
Unexported fields and fields that don't exist in both the encoded and decoded
struct types are ignored.  If a field with the same exported name exists in
both, their types must be compatible.

* Encoded arrays may be decoded into slices of a compatible element type.
Encoded slices may be decoded into arrays of a compatible element type, as long
as the array has at least enough capacity to store all encoded items.

User-defined coders work similarly to gob, but are a bit more general.  Add
VomEncode() and VomDecode() methods to your type like so:

  type Foo struct { a int }
  type fooVom string

  // VomEncode converts Foo into any value that is encodable via the regular vom
  // rules.  This example uses fooVom which is of type string, but fooVom may be
  // replaced with any type.  E.g. you might use []byte to use your own
  // marshaling format, or use an unexported struct with exported fields to
  // special-case unexported fields or unhandled types (chan, func).
  //
  // VomEncode typically takes a base (non-pointer) receiver so that base values
  // may be encoded, but may also take a pointer receiver.
  func (f Foo) VomEncode() (fooVom, error) {
    return fooVom(strconv.Itoa(f.a)), nil
  }

  // VomDecode fills in Foo via side-effect given fv.  This example uses fooVom
  // which is of type string, but just like for VomEncode, fooVom may be
  // replaced with any type.  The type used in VomEncode should be the same as
  // the type used in VomDecode.
  //
  // VomDecode must take a pointer receiver so that it can update the receiver
  // via side-effect.
  func (f *Foo) VomDecode(fv fooVom) error {
    var err error
    f.a, err = strconv.Atoi(string(fv))
    return err
  }

The advantage of this scheme over gob (which requires coding into []byte) is
that it's easier for common cases where you just need to translate your object
into a slightly different format for marshaling.  In addition type information
for these simple translations is retained using the regular vom rules.  And if
you really need a hand-crafted marshaling format, you can still use []byte.

As a convenience, we backoff to using GobEncode and GobDecode if they are
defined; some system types have built-in support for gob and it's nice to
automatically get reasonable behavior in vom.  We only use the methods if both
are defined and use "[]byte" as the arg, as per gob requirements.

Wire protocol.  Type ids are either in the bootstrap range (less than 65), or
are uniquely assigned by the encoder and received by the decoder.

  VOM:
    Message*
  Message:
    (TypeDef | PrimitiveValue | TypedValue)
  TypeDef: #WireType is one of {Meta,Slice,Array,Map,Struct,Ptr}Type#
    int(-typeID) uint(len(Message)) encoded WireType
  PrimitiveValue:
    int(+typeID) primitive
  TypedValue:
    int(+typeID) uint(len(Message)) Value
  Value:
    primitive | SliceV | ArrayV | MapV | StructV | PtrV | InterfaceV
  SliceV:
    uint(len) Value*len
  ArrayV:
    Value*len
  MapV:
    uint(len) (Value Value)*len
  StructV: #initial field index -1, field 0 has delta 1, zero fields not encoded#
    (uint(fieldDelta) Value)* uint(0)
  PtrV:
    int(0) #nil# | int(-refID) Value #value def# | int(+refID) #value ref#
  InterfaceV:
    int(0) #nil# | int(+typeID) Value

The grammar for the vom wire protocol is represented above; it is similar to the
gob wire protocol, but not interchangeable.  In particular the vom protocol
guarantees that value messages never contain type information, handles
interfaces in a simpler and more regular fashion, removes some redundancy wrt
non-struct values and type definitions, and uses different bootstrap type ids.

Primitive value wire encoding.  Other than bytes and bools, all other primitives
are identical to gob and variable-sized.

  Bytes:   Encode as 1 byte.
  Bools:   Encode as 1 byte where 0 is false and everything else is true.
  Uints:   Encode as 1 byte if less than 128, otherwise encode the negated byte
           length in 1 byte followed by the value in big-endian byte order.
  Ints:    Move sign bit from high bit to low bit and complement negative
           numbers (so +/- numbers only differ in sign), encoded as uint.
  Floats:  64-bit IEEE 754, byte reversed (so small numbers have lots of 0 high
           bits), encoded as uint.
  Strings: Encode as uint byte count followed by that many uninterpreted bytes.

Bootstrap type ids (different from gob):

	// Basic types
	typeIDInterface TypeID = 1 // 0x2 == STX
	typeIDBool      TypeID = 2 // 0x4 == EOT
	typeIDString    TypeID = 3 // 0x6 == ACK
	typeIDByteSlice TypeID = 4 // 0x8 == '\b'
	//              TypeID = 5 is 0xa == '\n', reserved.
	//              TypeID = 6 is 0xc == '\f', reserved.
	// Composite types, allowing users to define their own types.
	typeIDMetaType   TypeID = 8  // 0x10 == DLE
	typeIDSliceType  TypeID = 9  // 0x12 == DC2
	typeIDArrayType  TypeID = 10 // 0x14 == DC4
	typeIDMapType    TypeID = 11 // 0x16 == SYN
	typeIDStructType TypeID = 12 // 0x18 == CAN
	typeIDPtrType    TypeID = 13 // 0x1a == SUB
	//               TypeID = 16 is 0x20 == ' ', reserved.
	//               TypeID = 20 is 0x28 == '(', reserved.
	// Float types
	typeIDFloat   TypeID = 24 // 0x30 == '0'
	typeIDFloat32 TypeID = 25 // 0x32 == '1'
	typeIDFloat64 TypeID = 26 // 0x34 == '2'
	//            TypeID = 30 is 0x3c == '<', reserved.
	//            TypeID = 31 is 0x3e == '>', reserved.
	// Int types
	//          TypeID = 32 is 0x40 == '@', reserved.
	typeIDInt   TypeID = 33 // 0x42 == 'B'
	typeIDInt8  TypeID = 34 // 0x44 == 'D'
	typeIDInt16 TypeID = 35 // 0x46 == 'F'
	typeIDInt32 TypeID = 36 // 0x48 == 'H'
	typeIDInt64 TypeID = 37 // 0x4a == 'J'
	// Uint types
	typeIDUintptr TypeID = 48 // 0x60 == '`'
	typeIDUint    TypeID = 49 // 0x62 == 'b'
	typeIDUint8   TypeID = 50 // 0x64 == 'd'
	typeIDUint16  TypeID = 51 // 0x66 == 'f'
	typeIDUint32  TypeID = 52 // 0x68 == 'h'
	typeIDUint64  TypeID = 53 // 0x6a == 'j'
	// The first TypeID we assign to user types.
	typeIDFirst TypeID = 65

Differences between gob and vom:

* Gob requires that if you encode an interface value, you must decode into an
interface value; if you encode a concrete value, you must decode into a concrete
value.  Vom has no such requirement - you can decode into any value as long as
its type is compatible with the encoded value.

* Gob doesn't allow full numeric type compatibility - signed integers must be
decoded into signed integers, floats must be decoded into floats.

* Gob doesn't reconstruct pointer sharing present in DAGs or graphs, and can't
encode cyclic structures.

* Gob type registration forces you to pick a single type for a given base type;
e.g. you can only pick one of Foo, *Foo, **Foo, and the type you pick will be
used when decoding into a nil interface.  Vom forces you to register the base
type without any pointers (e.g. Foo), but will faithfully reproduce any number
of pointers when decoding into a nil interface.

* Gob user-defined encoders are restricted to output []byte.

* Gob encodes interfaces in a recursive format with weird framing, such that
sometimes "value messages" may contain embedded type information.  Vom messages
are guaranteed to be either type definitions or encoded values; we never mix
type and value information.

For more details see http://goto/veyron:vom
*/
package vom

// TODO(toddw): Add examples to the documentation above.
