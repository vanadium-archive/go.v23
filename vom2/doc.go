/*
Package vom2 implements Veyron Object Marshaling, a serialization protocol
similar to the encoding/gob package.  Vom is used in Veyron to enable
interchange of user-defined data structures across networks, languages and
storage systems.

The vom core API is almost identical to encoding/gob.  To marshal objects create
an Encoder and present it with a series of values.  To unmarshal objects create
a Decoder and retrieve values.  The implementation creates a possibly stateful
stream of messages between the Encoder and Decoder.

Vom supports a limited subset of values; not all Go values are faithfully
represented.  The following features are not supported:
  Pointers
  Sized numerics (e.g. int32, float64)
  Arrays
  Channels
  Functions

The types of the encoded and decoded values need not be identical, they only
need to be compatible.  Here are some compatibility rules:

+ Numeric values (ints, uints and floats) are compatible if the encoded value
can be represented without loss; e.g. an encoded float(1.0) may be decoded into
a value of type uint, but float(-1.0) and float(1.1) may not.

+ String values may always be decoded into bytes values.  Bytes value may be
decoded into string values, iff the bytes only contain valid UTF-8.

+ Enum values may always be decoded into string or bytes values.  String or
bytes values may be decoded into enum values, iff the value represents a valid
enum label.

+ Values of the same composite types are compatible if their contained values
are compatible, as specified by the other rules.  E.g. map[float]string is
compatible with map[uint]bytes as long as the encoded float keys are unsigned
integers.

+ Struct values are compatible based on their fields, identified by name.  If a
field with the same name exists in both, their values must be compatible.
Fields with names that exist in one struct but not the other are silently
ignored.

+ Struct values are compatible with maps with string keys, by assuming the map
represents a struct and applying the regular struct compatibility rules.  Struct
values may always be decoded into map[string]any; map[string]any may be decoded
into a struct iff the fields with the same name/key are compatible.  The
restricted form map[string]T is only compatible with structs with fields of type
T.

Vom supports two interchange formats: a binary format that is compact and fast,
and a JSON format that is simpler but slower.  The binary format is
self-describing; all type information describing the encoded values is present
in the encoded stream.  The JSON format prefers idiomatic JSON at the expense of
losing some of the type information.
*/
package vom2

/*
TODO: Describe user-defined coders (VomEncode?)
TODO: Describe wire format, something like this:

Wire protocol. // IMPLEMENTED PROTOCOL

The protocol consists of a stream of messages, where each message describes
either a type or a value.  All values are typed.  Here's the protocol grammar:
  VOM:
    (TypeMsg | ValueMsg)*
  TypeMsg:
    -typeID len(TypeMsg) encoded_WireType
  ValueMsg:
    +typeID primitive
  | +typeID len(ValueMsg) CompositeV
  Value:
    primitive |  CompositeV
  CompositeV:
    ArrayV | ListV | SetV | MapV | StructV | OneOfV | OptionalV | AnyV
  ArrayV:
    Value*len
  ListV:
    len Value*len
  SetV:
    len Value*len
  MapV:
    len (Value Value)*len
  StructV:
    (index Value)* 0  // index is the 1-based field index
  OneOfV:
    index Value       // index is the 1-based field index
  OptionalV:
    0 // nil
  | 1 Value
    // The 1 prefix before the value is to ensure the decoder can work
    // correctly; otherwise it wouldn't know what type to decode into.  This
    // will will be fixed with the new protocol below, which has a special
    // NIL_VALUE flag that's disjoint from all valid values.
  AnyV:
    0 // nil
  | +typeID Value



Wire protocol. // NEW PROTOCOL, NOT IMPLEMENTED YET

The protocol consists of a stream of messages, where each message describes
either a type or a value.  All values are typed.  Here's the protocol grammar:
  VOM:
    Message*
  Message:
    (TypeMsg | ValueMsg)
  TypeMsg:
    TypeID WireType
  ValueMsg:
    TypeID primitive
  | TypeID CompositeV
  Value:
    primitive |  CompositeV
  CompositeV:
    ArrayV | ListV | SetV | MapV | StructV | OneOfV | AnyV
  ArrayV:
    len Value*len // TODO(toddw): len only used to distinguish NIL_VALUE
  ListV:
    len Value*len
  SetV:
    len Value*len
  MapV:
    len (Value Value)*len
  StructV:
    (index Value)* END  // index is the 0-based field index
  OneOfV:
    index Value         // index is the 0-based field index
  AnyV:
    typeID Value

TODO(toddw): We need the message lengths for fast binary->binary transcoding.

The basis for the encoding is a variable-length unsigned integer (var256), with
a max size of 256 bits (32 bytes).  This is a byte-based encoding.  The first
byte encodes values 0x00...0x7F verbatim.  Otherwise it encodes the length of
the value, and the value is encoded in the subsequent bytes in big-endian order.
In addition we have space for 32 flags and 64 reserved entries.

The var256 encoding tries to strike a balance between the coding size and
performance; we try to not be overtly wasteful of space, but still keep the
format simple to encode and decode.

  First byte of var256:
  |7|6|5|4|3|2|1|0|
  |---------------|
  |0| Single value| 0x00...0x7F Single-byte value (0...127)
  -----------------
  |1|0|x|x|x|x|x|x| 0x80...0xBF Reserved (64 entries)
  |1|1|0| Flags   | 0xC0...0xDF Flags    (32 entries)
  |1|1|1| Len     | 0xE0...0xFF Multi-byte length (FF=1 byte, FE=2 bytes, ...)
  -----------------             (i.e. the length is -Len)

The encoding of the value, flags and reserved entries are all disjoint from each
other; each var256 can hold either a single 256 bit value, or 5 flag bits, or 6
reserved bits.  The encoding favors small values; values less than 0x7F, flags
and reserved entries are all encoded in one byte.

The primitives are all encoded using var256:
  o Unsigned: Verbatim.
  o Signed :  Low bit 0 for positive and 1 for negative, and indicates whether
              to complement the other bits to recover the signed value.
  o Float:    Byte-reversed ieee754 64-bit float.
  o Complex:  Two floats, real and imaginary.
  o String:   Byte count followed by uninterpreted bytes.

Flags are used to represent special properties and values:
  0xC0  // NIL_VALUE - represents any(nil), a non-exitent value.
  0xC1  // END       - end sentry, e.g. used for structs
  ...
TODO(toddw): Add a flag indicating there is a local TypeID table for Any types.

The first byte of each message takes advantage of the var256 flags and reserved
entries, to make common encodings smaller, but still easy to decode.  The
assumption is that values will be encoded more frequently than types; we expect
values of the same type to be encoded repeatedly.  Conceptually the first byte
needs to distinguish TypeMsg from ValueMsg, and also tell us the TypeID.

First byte of each message:
  |7|6|5|4|3|2|1|0|
  |---------------|
  |0|0|0|0|0|0|0|0| Reserved (1 entry   0x00)
  |0|1|x|x|x|0|0|0| Reserved (8 entries 0x40, 48, 50, 58, 60, 68, 70, 78)
  |0|0|1|x|x|0|0|0| Reserved (4 entries 0x20, 28, 30, 38)
  |0|0|0|1|0|0|0|0| TypeMsg  (0x10, TypeID encoded next, then WireType)
  |0|0|0|0|1|0|0|0| ValueMsg bool false (0x08)
  |0|0|0|1|1|0|0|0| ValueMsg bool true  (0x18)
  |0| StrLen|0|1|0| ValueMsg small string len (0...15)
  |0| Uint  |1|0|0| ValueMsg small uint (0...15)
  |0| Int   |1|1|0| ValueMsg small int (-8...7)
  |0| TypeID    |1| ValueMsg (6-bit built-in TypeID)
  -----------------
  |1|0|   TypeID  | ValueMsg (6-bit user TypeID)
  |1|1|0| Flags   | Flags    (32 entries 0xC0...0xDF)
  |1|1|1| Len     | Multi-byte length (FF=1 byte, FE=2 bytes, ..., F8=8 bytes)
  -----------------                   (i.e. the length is -Len)

If the first byte is 0x10, this is a TypeMsg, and we encode the TypeID next,
followed by the WireType.  The WireType is simply encoded as a regular value,
using the protocol described in the grammar above.

Otherwise this is a ValueMsg.  We encode small bool, uint and int values that
fit into 4 bits directly into the first byte, along with their TypeID.  For
small strings with len <= 15, we encode the length into the first byte, followed
by the bytes of the string value; empty strings are a single byte 0x02.

The first byte of the ValueMsg also contains TypeIDs [0...127], where the
built-in TypeIDs occupy [0...63], and user-defined TypeIDs start at 64.
User-defined TypeIDs larger than 127 are encoded as regular multi-byte var256.

TODO(toddw): For small value encoding to be useful, we'll want to use it for all
values that can fit, but we'll be dropping the sizes of int and uint, and the
type names.  Now that OneOf is labeled, the only issue is Any.  And now that we
have Signature with type information, maybe we can drop the type names
regularly, and only send them when the Signature says "Any".  This also impacts
where we perform value conversions - does it happen on the server or the client?

*/
