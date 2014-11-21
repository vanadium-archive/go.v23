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

// THIS IS THE NEW PROTOCOL, IT'S NOT IMPLEMENTED YET.

Wire protocol.  Type ids are either in the bootstrap range (less than 65), or
are uniquely assigned by the encoder and received by the decoder.

  VOM:
    (TypeMsg | ValueMsg)*
  TypeMsg:
    -typeID len(TypeMsg) encoded_WireType
  ValueMsg:
    +typeID primitive
  | +typeID len(Msg) CompositeV
  Value:
    primitive |  CompositeV
  CompositeV:
    ListV | SetV | MapV | StructV | OneOfV | AnyV
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
    +typeID Value

  primitive: // variable-length unsigned integer
    00-7F // 1-byte value in range 0-127
    80-8F // Reserved flags
    90-FF // Multi-byte value, FF=2 bytes, FE=3 bytes, ...

  flags:
    80  // NOEXIST - used to indicate a non-existent optional / any value.
    81  // END     - used to terminate structs.
*/
