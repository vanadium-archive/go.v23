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

Wire protocol.  Type ids are either in the bootstrap range (less than 65), or
are uniquely assigned by the encoder and received by the decoder.

  VOM:
    Message*
  Message:
    (TypeDef | ValueDef)
  TypeDef: #WireType is one of {Slice,Map,Struct}Type#
    int(-typeID) uint(len(Message)) encoded_WireType
  ValueDef:
    int(+typeID) primitive
  | int(+typeID) uint(len(Message)) CompositeValue
  CompositeValue:
    SliceV | MapV | StructV | AnyV
  Value:
    primitive |  CompositeValue
  SliceV:
    uint(len) Value*len
  MapV:
    uint(len) (Value Value)*len
  StructV: #initial field index -1, field 0 has delta 1, zero fields not encoded#
    (uint(fieldDelta) Value)* uint(0)
  AnyV:
    int(0) #nil# | int(+typeID) Value

//////////
type encBinary struct {}
func (e *encBinary) encode(w io.Writer) error

func (e *encBinary) encType(t *Type) error

func (e *encBinary) encNil() error
func (e *encBinary) encBool(v bool) error
func (e *encBinary) encUint(v uint64) error
func (e *encBinary) encInt(v int64) error
func (e *encBinary) encFloat(v float64) error
func (e *encBinary) encString(v string) error

func (e *encBinary) encStartSlice(len int) error
func (e *encBinary) encFinishSlice() error

func (e *encBinary) encStartMap(len int) error
func (e *encBinary) encFinishMap() error

func (e *encBinary) encStartStruct() error
func (e *encBinary) encStructField(index int) error
func (e *encBinary) encFinishStruct() error

type decBinary struct {}
func (d *decBinary) decode(r io.Reader) error

func (d *decBinary) decType(t *Type) error

func (d *decBinary) decNil() error
func (d *decBinary) decBool(v bool) error
func (d *decBinary) decUint(v uint64) error
func (d *decBinary) decInt(v int64) error
func (d *decBinary) decFloat(v float64) error
func (d *decBinary) decString(v string) error

func (d *decBinary) decStartSlice(len int) error
func (d *decBinary) decFinishSlice() error

func (d *decBinary) decStartMap(len int) error
func (d *decBinary) decFinishMap() error

func (d *decBinary) decStartStruct() error
func (d *decBinary) decStructField(index int) error
func (d *decBinary) decFinishStruct() error
*/
