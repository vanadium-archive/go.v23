package arith

import (
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"testing"

	_ "veyron/lib/testutil"

	"veyron2/ipc"
	"veyron2/naming"
	"veyron2/rt"
	"veyron2/vdl"
	"veyron2/vdl/test_base"
	"veyron2/wiretype"
)

var generatedError = errors.New("generated error")

func newClientServer() (ipc.Client, ipc.Server) {
	r := rt.Init()
	c, err := r.NewClient()
	if err != nil {
		panic(err)
	}
	s, err := r.NewServer()
	if err != nil {
		panic(err)
	}
	return c, s
}

// serverArith implements the Arith interface.
type serverArith struct{}

func (*serverArith) Add(_ ipc.Context, A, B int32) (int32, error) {
	return A + B, nil
}

func (*serverArith) DivMod(_ ipc.Context, A, B int32) (int32, int32, error) {
	return A / B, A % B, nil
}

func (*serverArith) Sub(_ ipc.Context, args test_base.Args) (int32, error) {
	return args.A - args.B, nil
}

func (*serverArith) Mul(_ ipc.Context, nestedArgs test_base.NestedArgs) (int32, error) {
	return nestedArgs.Args.A * nestedArgs.Args.B, nil
}

func (*serverArith) Count(_ ipc.Context, Start int32, Stream ArithServiceCountStream) error {
	const kNum = 1000
	for i := int32(0); i < kNum; i++ {
		if err := Stream.Send(Start + i); err != nil {
			return err
		}
	}
	return nil
}

func (*serverArith) StreamingAdd(_ ipc.Context, Stream ArithServiceStreamingAddStream) (int32, error) {
	var total int32
	for {
		value, err := Stream.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			return total, err
		}
		total += value
		Stream.Send(total)
	}
	return total, nil
}

func (*serverArith) GenError(_ ipc.Context) error {
	return generatedError
}

func (*serverArith) QuoteAny(_ ipc.Context, any vdl.Any) (vdl.Any, error) {
	return fmt.Sprintf("'%v'", any), nil
}

type serverCalculator struct {
	serverArith
}

func (*serverCalculator) Sine(_ ipc.Context, angle float64) (float64, error) {
	return math.Sin(angle), nil
}

func (*serverCalculator) Cosine(_ ipc.Context, angle float64) (float64, error) {
	return math.Cos(angle), nil
}

func (*serverCalculator) Exp(_ ipc.Context, x float64) (float64, error) {
	return math.Exp(x), nil
}

func (*serverCalculator) On(_ ipc.Context) error {
	return nil
}

func (*serverCalculator) Off(_ ipc.Context) error {
	return nil
}

func TestCalculator(t *testing.T) {
	client, server := newClientServer()
	if err := server.Register("math/calculator", ipc.SoloDispatcher(NewServerCalculator(&serverCalculator{}), nil)); err != nil {
		t.Fatal(err)
	}
	ep, err := server.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	name := naming.JoinAddressName(ep.String(), "math/calculator")
	// Synchronous calls
	calculator, err := BindCalculator(name, client)
	if err != nil {
		t.Fatalf("Bind: got %q but expected no error", err)
	}
	sine, err := calculator.Sine(0)
	if err != nil {
		t.Errorf("Sine: got %q but expected no error", err)
	}
	if sine != 0 {
		t.Errorf("Sine: expected 0 got %f", sine)
	}
	cosine, err := calculator.Cosine(0)
	if err != nil {
		t.Errorf("Cosine: got %q but expected no error", err)
	}
	if cosine != 1 {
		t.Errorf("Cosine: expected 1 got %f", cosine)
	}

	arith, err := BindArith(name, client)
	if err != nil {
		t.Errorf("Bind: got %q but expected no error", err)
	}
	sum, err := arith.Add(7, 8)
	if err != nil {
		t.Errorf("Add: got %q but expected no error", err)
	}
	if sum != 15 {
		t.Errorf("Add: expected 15 got %d", sum)
	}
	arith = calculator
	sum, err = arith.Add(7, 8)
	if err != nil {
		t.Errorf("Add: got %q but expected no error", err)
	}
	if sum != 15 {
		t.Errorf("Add: expected 15 got %d", sum)
	}

	trig, err := BindTrigonometry(name, client)
	if err != nil {
		t.Errorf("Bind: got %q but expected no error", err)
	}
	cosine, err = trig.Cosine(0)
	if err != nil {
		t.Errorf("Cosine: got %q but expected no error", err)
	}
	if cosine != 1 {
		t.Errorf("Cosine: expected 1 got %f", cosine)
	}

	// Test auto-generated methods.
	serverStub := NewServerCalculator(&serverCalculator{}).(*ServerStubCalculator)

	tagTests := []struct {
		method   string
		expected []interface{}
	}{
		{"GenError", []interface{}{"foo", "barz", "hello", int32(129), uint64(36)}},
		{"Off", []interface{}{"offtag"}},
	}
	for _, tagTest := range tagTests {
		tags, err := serverStub.GetMethodTags(nil, tagTest.method)
		if err != nil {
			t.Errorf("Error calling GetMethodTags(%q): ", tagTest.method, err)
		}
		if !reflect.DeepEqual(tags, tagTest.expected) {
			t.Errorf("GetMethodTags(%q): got %v but expected %v", tagTest.method, tags, tagTest.expected)
		}
	}
	signature, err := serverStub.Signature(nil)
	expectedSignature := map[string]ipc.MethodSignature{
		"Add": ipc.MethodSignature{
			InArgs: []ipc.MethodArgument{
				{Name: "a", Type: 36},
				{Name: "b", Type: 36},
			},
			OutArgs: []ipc.MethodArgument{
				{Name: "", Type: 36},
				{Name: "", Type: 66},
			},
		},
		"DivMod": ipc.MethodSignature{
			InArgs: []ipc.MethodArgument{
				{Name: "a", Type: 36},
				{Name: "b", Type: 36},
			},
			OutArgs: []ipc.MethodArgument{
				{Name: "quot", Type: 36},
				{Name: "rem", Type: 36},
				{Name: "err", Type: 66},
			},
		},
		"Sub": ipc.MethodSignature{
			InArgs: []ipc.MethodArgument{
				{Name: "args", Type: 67},
			},
			OutArgs: []ipc.MethodArgument{
				{Name: "", Type: 36},
				{Name: "", Type: 66},
			},
		},
		"Mul": ipc.MethodSignature{
			InArgs: []ipc.MethodArgument{
				{Name: "nested", Type: 68},
			},
			OutArgs: []ipc.MethodArgument{
				{Name: "", Type: 36},
				{Name: "", Type: 66},
			},
		},
		"GenError": ipc.MethodSignature{
			InArgs: []ipc.MethodArgument{},
			OutArgs: []ipc.MethodArgument{
				{Name: "", Type: 66},
			},
		},
		"Count": ipc.MethodSignature{
			InArgs: []ipc.MethodArgument{
				{Name: "Start", Type: 36},
			},
			OutArgs: []ipc.MethodArgument{
				{Name: "", Type: 66},
			},

			OutStream: 36,
		},
		"StreamingAdd": ipc.MethodSignature{
			InArgs: []ipc.MethodArgument{},
			OutArgs: []ipc.MethodArgument{
				{Name: "total", Type: 36},
				{Name: "err", Type: 66},
			},
			InStream:  36,
			OutStream: 36,
		},
		"QuoteAny": ipc.MethodSignature{
			InArgs: []ipc.MethodArgument{
				{Name: "a", Type: 69},
			},
			OutArgs: []ipc.MethodArgument{
				{Name: "", Type: 69},
				{Name: "", Type: 66},
			},
		},
		"Sine": ipc.MethodSignature{
			InArgs: []ipc.MethodArgument{
				{Name: "angle", Type: 26},
			},
			OutArgs: []ipc.MethodArgument{
				{Name: "", Type: 26},
				{Name: "", Type: 70},
			},
		},
		"Cosine": ipc.MethodSignature{
			InArgs: []ipc.MethodArgument{
				{Name: "angle", Type: 26},
			},
			OutArgs: []ipc.MethodArgument{
				{Name: "", Type: 26},
				{Name: "", Type: 70},
			},
		},
		"Exp": ipc.MethodSignature{
			InArgs: []ipc.MethodArgument{
				{Name: "x", Type: 26},
			},
			OutArgs: []ipc.MethodArgument{
				{Name: "", Type: 26},
				{Name: "", Type: 71},
			},
		},
		"On": ipc.MethodSignature{
			InArgs: []ipc.MethodArgument{},
			OutArgs: []ipc.MethodArgument{
				{Name: "", Type: 65},
			},
		},
		"Off": ipc.MethodSignature{
			InArgs: []ipc.MethodArgument{},
			OutArgs: []ipc.MethodArgument{
				{Name: "", Type: 65},
			},
		},
	}
	if !reflect.DeepEqual(signature.Methods, expectedSignature) {
		t.Errorf("Signature Methods: got %v but expected %v", signature.Methods, expectedSignature)
	}

	expectedTypeDefs := []vdl.Any{
		// Calculator:
		wiretype.NamedPrimitiveType{1, "error", nil},
		// Arith:
		wiretype.NamedPrimitiveType{1, "error", nil},
		wiretype.StructType{[]wiretype.FieldType{wiretype.FieldType{36, "A"},
			wiretype.FieldType{36, "B"}}, "veyron2/vdl/test_base.Args", nil},
		wiretype.StructType{[]wiretype.FieldType{wiretype.FieldType{66, "Args"}}, "veyron2/vdl/test_base.NestedArgs", nil},
		wiretype.NamedPrimitiveType{1, "anydata", nil},
		// Trig:
		wiretype.NamedPrimitiveType{1, "error", nil},
		// Exp:
		wiretype.NamedPrimitiveType{1, "error", nil},
	}

	if !reflect.DeepEqual(signature.TypeDefs, expectedTypeDefs) {
		t.Errorf("Signature TypeDefs: got %v but expected %v", signature.TypeDefs, expectedTypeDefs)
	}

}

func TestArith(t *testing.T) {
	// TODO(bprosnitz) Split this test up -- it is quite long and hard to debug.

	// We try a few types of dispatchers on the server side, to verify that
	// anything dispatching to Arith or an interface embedding Arith (like
	// Calculator) works for a client looking to talk to an Arith service.
	dispatchers := []ipc.Dispatcher{
		ipc.SoloDispatcher(NewServerArith(&serverArith{}), nil),
		ipc.SoloDispatcher(NewServerArith(&serverCalculator{}), nil),
		ipc.SoloDispatcher(NewServerCalculator(&serverCalculator{}), nil),
	}

	client, server := newClientServer()
	ep, err := server.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	name := naming.JoinAddressName(ep.String(), "")
	for i, disp := range dispatchers {
		dispSuffix := fmt.Sprintf("arith%d", i)
		if err := server.Register(dispSuffix, disp); err != nil {
			t.Fatalf("%q: %v", dispSuffix, err)
		}
		// Synchronous calls
		arith, err := BindArith(naming.Join(name, dispSuffix), client)
		if err != nil {
			t.Errorf("Bind: got %q but expected no error", err)
		}

		sum, err := arith.Add(7, 8)
		if err != nil {
			t.Errorf("Add: got %q but expected no error", err)
		}
		if sum != 15 {
			t.Errorf("Add: expected 15 got %d", sum)
		}
		q, r, err := arith.DivMod(7, 3)
		if err != nil {
			t.Errorf("DivMod: got %q but expected no error", err)
		}
		if q != 2 || r != 1 {
			t.Errorf("DivMod: expected (2,1) got (%d,%d)", q, r)
		}
		diff, err := arith.Sub(test_base.Args{7, 8})
		if err != nil {
			t.Errorf("Sub: got %q but expected no error", err)
		}
		if diff != -1 {
			t.Errorf("Sub: got %d, expected -1", diff)
		}
		prod, err := arith.Mul(test_base.NestedArgs{test_base.Args{7, 8}})
		if err != nil {
			t.Errorf("Mul: got %q, but expected no error", err)
		}
		if prod != 56 {
			t.Errorf("Sub: got %d, expected 56", prod)
		}
		stream, err := arith.Count(35)
		if err != nil {
			t.Fatalf("error while executing Count %v", err)
		}

		for i := int32(0); i < 1000; i++ {
			val, err := stream.Recv()
			if err != nil {
				t.Errorf("Error getting value %v", err)
			}
			if val != 35+i {
				t.Errorf("Expected value %d, got %d", 35+i, val)
			}
		}
		if _, err := stream.Recv(); err != io.EOF {
			t.Errorf("Reply stream should have been closed %v", err)
		}

		if err := stream.Finish(); err != nil {
			t.Errorf("Count failed with %v", err)
		}

		addStream, err := arith.StreamingAdd()

		go func() {
			for i := int32(0); i < 100; i++ {
				if err := addStream.Send(i); err != nil {
					t.Errorf("Send error %v", err)
				}
			}
			if err := addStream.CloseSend(); err != nil {
				t.Errorf("CloseSend error %v", err)
			}
		}()

		var expectedSum int32
		for i := int32(0); i < 100; i++ {
			expectedSum += i
			value, err := addStream.Recv()
			if err != nil {
				t.Errorf("Error getting value %v", err)
			} else if value != expectedSum {
				t.Errorf("Got %d but expected %d", value, expectedSum)
			}
		}

		if _, err := addStream.Recv(); err != io.EOF {
			t.Errorf("Reply stream should have been closed %v", err)
		}

		total, err := addStream.Finish()

		if err != nil {
			t.Errorf("Count failed with %v", err)
		}

		if total != expectedSum {
			t.Errorf("Got %d but expexted %d", total, expectedSum)
		}

		if err := arith.GenError(); err == nil {
			t.Errorf("GenError: got %v but expected %v", err, generatedError)
		}

		// Client-side stubs

		clientStub := arith.(*clientStubArith)

		tags, err := clientStub.GetMethodTags("GenError")
		expectedTags := []interface{}{"foo", "barz", "hello", int32(129), uint64(36)}
		if err != nil {
			t.Error(`Error calling GetMethodTags("GenError"): `, err)
		}
		if !reflect.DeepEqual(tags, expectedTags) {
			t.Errorf("GetMethodTags: got %v but expected %v", tags, expectedTags)
		}

		if _, err := clientStub.Signature(nil); err != nil {
			// TODO(bprosnitz) Check that the signature is the expected signature.
			t.Errorf("Failed to get signature from client: %v", err)
		}

		// Server-side stubs

		serverStub := NewServerArith(&serverArith{}).(*ServerStubArith)

		tags, err = serverStub.GetMethodTags(nil, "GenError")
		expectedTags = []interface{}{"foo", "barz", "hello", int32(129), uint64(36)}
		if err != nil {
			t.Error(`Error calling GetMethodTags("GenError"): `, err)
		}
		if !reflect.DeepEqual(tags, expectedTags) {
			t.Errorf("GetMethodTags: got %v but expected %v", tags, expectedTags)
		}

		signature, err := serverStub.Signature(nil)
		expectedSignature := map[string]ipc.MethodSignature{
			"Add": ipc.MethodSignature{
				InArgs: []ipc.MethodArgument{
					{Name: "a", Type: 36},
					{Name: "b", Type: 36},
				},
				OutArgs: []ipc.MethodArgument{
					{Name: "", Type: 36},
					{Name: "", Type: 65},
				},
			},
			"DivMod": ipc.MethodSignature{
				InArgs: []ipc.MethodArgument{
					{Name: "a", Type: 36},
					{Name: "b", Type: 36},
				},
				OutArgs: []ipc.MethodArgument{
					{Name: "quot", Type: 36},
					{Name: "rem", Type: 36},
					{Name: "err", Type: 65},
				},
			},
			"Sub": ipc.MethodSignature{
				InArgs: []ipc.MethodArgument{
					{Name: "args", Type: 66},
				},
				OutArgs: []ipc.MethodArgument{
					{Name: "", Type: 36},
					{Name: "", Type: 65},
				},
			},
			"Mul": ipc.MethodSignature{
				InArgs: []ipc.MethodArgument{
					{Name: "nested", Type: 67},
				},
				OutArgs: []ipc.MethodArgument{
					{Name: "", Type: 36},
					{Name: "", Type: 65},
				},
			},
			"GenError": ipc.MethodSignature{
				InArgs: []ipc.MethodArgument{},
				OutArgs: []ipc.MethodArgument{
					{Name: "", Type: 65},
				},
			},
			"Count": ipc.MethodSignature{
				InArgs: []ipc.MethodArgument{
					{Name: "Start", Type: 36},
				},
				OutArgs: []ipc.MethodArgument{
					{Name: "", Type: 65},
				},

				OutStream: 36,
			},
			"StreamingAdd": ipc.MethodSignature{
				InArgs: []ipc.MethodArgument{},
				OutArgs: []ipc.MethodArgument{
					{Name: "total", Type: 36},
					{Name: "err", Type: 65},
				},
				InStream:  36,
				OutStream: 36,
			},
			"QuoteAny": ipc.MethodSignature{
				InArgs: []ipc.MethodArgument{
					{Name: "a", Type: 68},
				},
				OutArgs: []ipc.MethodArgument{
					{Name: "", Type: 68},
					{Name: "", Type: 65},
				},
			},
		}
		if !reflect.DeepEqual(signature.Methods, expectedSignature) {
			t.Errorf("Signature Methods: got %v but expected %v", signature.Methods, expectedSignature)
		}

		expectedTypeDefs := []vdl.Any{
			wiretype.NamedPrimitiveType{1, "error", nil},
			wiretype.StructType{[]wiretype.FieldType{wiretype.FieldType{36, "A"}, wiretype.FieldType{36, "B"}}, "veyron2/vdl/test_base.Args", nil},
			wiretype.StructType{[]wiretype.FieldType{wiretype.FieldType{66, "Args"}}, "veyron2/vdl/test_base.NestedArgs", nil},
			wiretype.NamedPrimitiveType{1, "anydata", nil},
		}

		if !reflect.DeepEqual(signature.TypeDefs, expectedTypeDefs) {
			t.Errorf("Signature TypeDefs: got %v but expected %v", signature.TypeDefs, expectedTypeDefs)
		}
	}
}
