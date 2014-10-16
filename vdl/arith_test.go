package vdl_test

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"

	"veyron.io/veyron/veyron2/ipc"
	"veyron.io/veyron/veyron2/naming"
	"veyron.io/veyron/veyron2/rt"
	"veyron.io/veyron/veyron2/vdl/testdata/arith"
	"veyron.io/veyron/veyron2/vdl/testdata/base"
	"veyron.io/veyron/veyron2/vdl/vdlutil"
	"veyron.io/veyron/veyron2/wiretype"

	"veyron.io/veyron/veyron/profiles"
)

var generatedError = errors.New("generated error")

func newClient() ipc.Client {
	r := rt.Init()
	c, err := r.NewClient()
	if err != nil {
		panic(err)
	}
	return c
}

func newServer() ipc.Server {
	r := rt.Init()
	s, err := r.NewServer()
	if err != nil {
		panic(err)
	}
	return s
}

// serverArith implements the arith.Arith interface.
type serverArith struct{}

func (*serverArith) Add(_ ipc.ServerContext, A, B int32) (int32, error) {
	return A + B, nil
}

func (*serverArith) DivMod(_ ipc.ServerContext, A, B int32) (int32, int32, error) {
	return A / B, A % B, nil
}

func (*serverArith) Sub(_ ipc.ServerContext, args base.Args) (int32, error) {
	return args.A - args.B, nil
}

func (*serverArith) Mul(_ ipc.ServerContext, nestedArgs base.NestedArgs) (int32, error) {
	return nestedArgs.Args.A * nestedArgs.Args.B, nil
}

func (*serverArith) Count(_ ipc.ServerContext, Start int32, Stream arith.ArithServiceCountStream) error {
	const kNum = 1000
	sender := Stream.SendStream()
	for i := int32(0); i < kNum; i++ {
		if err := sender.Send(Start + i); err != nil {
			return err
		}
	}
	return nil
}

func (*serverArith) StreamingAdd(_ ipc.ServerContext, Stream arith.ArithServiceStreamingAddStream) (int32, error) {
	var total int32
	rStream := Stream.RecvStream()
	sender := Stream.SendStream()
	for rStream.Advance() {
		value := rStream.Value()
		total += value
		sender.Send(total)
	}
	return total, rStream.Err()
}

func (*serverArith) GenError(_ ipc.ServerContext) error {
	return generatedError
}

func (*serverArith) QuoteAny(_ ipc.ServerContext, any vdlutil.Any) (vdlutil.Any, error) {
	return fmt.Sprintf("'%v'", any), nil
}

type serverCalculator struct {
	serverArith
}

func (*serverCalculator) Sine(_ ipc.ServerContext, angle float64) (float64, error) {
	return math.Sin(angle), nil
}

func (*serverCalculator) Cosine(_ ipc.ServerContext, angle float64) (float64, error) {
	return math.Cos(angle), nil
}

func (*serverCalculator) Exp(_ ipc.ServerContext, x float64) (float64, error) {
	return math.Exp(x), nil
}

func (*serverCalculator) On(_ ipc.ServerContext) error {
	return nil
}

func (*serverCalculator) Off(_ ipc.ServerContext) error {
	return nil
}

func TestCalculator(t *testing.T) {
	client := newClient()
	server := newServer()
	if err := server.Serve("", ipc.LeafDispatcher(arith.NewServerCalculator(&serverCalculator{}), nil)); err != nil {
		t.Fatal(err)
	}
	ep, err := server.ListenX(profiles.LocalListenSpec)
	if err != nil {
		t.Fatal(err)
	}
	root := naming.JoinAddressName(ep.String(), "")
	ctx := rt.R().NewContext()
	// Synchronous calls
	calculator, err := arith.BindCalculator(root, client)
	if err != nil {
		t.Fatalf("Bind: got %q but expected no error", err)
	}
	sine, err := calculator.Sine(ctx, 0)
	if err != nil {
		t.Errorf("Sine: got %q but expected no error", err)
	}
	if sine != 0 {
		t.Errorf("Sine: expected 0 got %f", sine)
	}
	cosine, err := calculator.Cosine(ctx, 0)
	if err != nil {
		t.Errorf("Cosine: got %q but expected no error", err)
	}
	if cosine != 1 {
		t.Errorf("Cosine: expected 1 got %f", cosine)
	}

	ar, err := arith.BindArith(root, client)
	if err != nil {
		t.Errorf("Bind: got %q but expected no error", err)
	}
	sum, err := ar.Add(ctx, 7, 8)
	if err != nil {
		t.Errorf("Add: got %q but expected no error", err)
	}
	if sum != 15 {
		t.Errorf("Add: expected 15 got %d", sum)
	}
	ar = calculator
	sum, err = ar.Add(ctx, 7, 8)
	if err != nil {
		t.Errorf("Add: got %q but expected no error", err)
	}
	if sum != 15 {
		t.Errorf("Add: expected 15 got %d", sum)
	}

	trig, err := arith.BindTrigonometry(root, client)
	if err != nil {
		t.Errorf("Bind: got %q but expected no error", err)
	}
	cosine, err = trig.Cosine(ctx, 0)
	if err != nil {
		t.Errorf("Cosine: got %q but expected no error", err)
	}
	if cosine != 1 {
		t.Errorf("Cosine: expected 1 got %f", cosine)
	}

	// Test auto-generated methods.
	serverStub := arith.NewServerCalculator(&serverCalculator{}).(*arith.ServerStubCalculator)

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

	expectedTypeDefs := []vdlutil.Any{
		// Calculator:
		wiretype.NamedPrimitiveType{1, "error", nil},
		// Arith:
		wiretype.NamedPrimitiveType{1, "error", nil},
		wiretype.StructType{[]wiretype.FieldType{wiretype.FieldType{36, "A"},
			wiretype.FieldType{36, "B"}}, "veyron.io/veyron/veyron2/vdl/testdata/base.Args", nil},
		wiretype.StructType{[]wiretype.FieldType{wiretype.FieldType{67, "Args"}}, "veyron.io/veyron/veyron2/vdl/testdata/base.NestedArgs", nil},
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
		ipc.LeafDispatcher(arith.NewServerArith(&serverArith{}), nil),
		ipc.LeafDispatcher(arith.NewServerArith(&serverCalculator{}), nil),
		ipc.LeafDispatcher(arith.NewServerCalculator(&serverCalculator{}), nil),
	}

	client := newClient()
	ctx := rt.R().NewContext()

	for i, disp := range dispatchers {
		server := newServer()
		defer server.Stop()
		ep, err := server.ListenX(profiles.LocalListenSpec)
		if err != nil {
			t.Fatal(err)
		}
		root := naming.JoinAddressName(ep.String(), "")
		if err := server.Serve("", disp); err != nil {
			t.Fatalf("%d: %v", i, err)
		}
		// Synchronous calls
		ar, err := arith.BindArith(root, client)
		if err != nil {
			t.Errorf("Bind: got %q but expected no error", err)
		}

		sum, err := ar.Add(ctx, 7, 8)
		if err != nil {
			t.Errorf("Add: got %q but expected no error", err)
		}
		if sum != 15 {
			t.Errorf("Add: expected 15 got %d", sum)
		}
		q, r, err := ar.DivMod(ctx, 7, 3)
		if err != nil {
			t.Errorf("DivMod: got %q but expected no error", err)
		}
		if q != 2 || r != 1 {
			t.Errorf("DivMod: expected (2,1) got (%d,%d)", q, r)
		}
		diff, err := ar.Sub(ctx, base.Args{7, 8})
		if err != nil {
			t.Errorf("Sub: got %q but expected no error", err)
		}
		if diff != -1 {
			t.Errorf("Sub: got %d, expected -1", diff)
		}
		prod, err := ar.Mul(ctx, base.NestedArgs{base.Args{7, 8}})
		if err != nil {
			t.Errorf("Mul: got %q, but expected no error", err)
		}
		if prod != 56 {
			t.Errorf("Sub: got %d, expected 56", prod)
		}
		stream, err := ar.Count(ctx, 35)
		if err != nil {
			t.Fatalf("error while executing Count %v", err)
		}

		countIterator := stream.RecvStream()
		for i := int32(0); i < 1000; i++ {
			if !countIterator.Advance() {
				t.Errorf("Error getting value %v", countIterator.Err())
			}
			val := countIterator.Value()
			if val != 35+i {
				t.Errorf("Expected value %d, got %d", 35+i, val)
			}
		}
		if countIterator.Advance() || countIterator.Err() != nil {
			t.Errorf("Reply stream should have been closed %v", countIterator.Err())
		}

		if err := stream.Finish(); err != nil {
			t.Errorf("Count failed with %v", err)
		}

		addStream, err := ar.StreamingAdd(ctx)

		go func() {
			sender := addStream.SendStream()
			for i := int32(0); i < 100; i++ {
				if err := sender.Send(i); err != nil {
					t.Errorf("Send error %v", err)
				}
			}
			if err := sender.Close(); err != nil {
				t.Errorf("Close error %v", err)
			}
		}()

		var expectedSum int32
		rStream := addStream.RecvStream()
		for i := int32(0); i < 100; i++ {
			expectedSum += i
			if !rStream.Advance() {
				t.Errorf("Error getting value %v", rStream.Err())
			}
			value := rStream.Value()
			if value != expectedSum {
				t.Errorf("Got %d but expected %d", value, expectedSum)
			}
		}

		if rStream.Advance() || rStream.Err() != nil {
			t.Errorf("Reply stream should have been closed %v", rStream.Err())
		}

		total, err := addStream.Finish()

		if err != nil {
			t.Errorf("Count failed with %v", err)
		}

		if total != expectedSum {
			t.Errorf("Got %d but expexted %d", total, expectedSum)
		}

		if err := ar.GenError(ctx); err == nil {
			t.Errorf("GenError: got %v but expected %v", err, generatedError)
		}

		// Client-side stubs
		clientStub := ar.(ipc.UniversalServiceMethods)

		tags, err := clientStub.GetMethodTags(ctx, "GenError")
		expectedTags := []interface{}{"foo", "barz", "hello", int32(129), uint64(36)}
		if err != nil {
			t.Error(`Error calling GetMethodTags("GenError"): `, err)
		}
		if !reflect.DeepEqual(tags, expectedTags) {
			t.Errorf("GetMethodTags: got %v but expected %v", tags, expectedTags)
		}

		if _, err := clientStub.Signature(ctx); err != nil {
			// TODO(bprosnitz) Check that the signature is the expected signature.
			t.Errorf("Failed to get signature from client: %v", err)
		}

		// Server-side stubs

		serverStub := arith.NewServerArith(&serverArith{}).(*arith.ServerStubArith)

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

		expectedTypeDefs := []vdlutil.Any{
			wiretype.NamedPrimitiveType{1, "error", nil},
			wiretype.StructType{[]wiretype.FieldType{wiretype.FieldType{36, "A"}, wiretype.FieldType{36, "B"}}, "veyron.io/veyron/veyron2/vdl/testdata/base.Args", nil},
			wiretype.StructType{[]wiretype.FieldType{wiretype.FieldType{66, "Args"}}, "veyron.io/veyron/veyron2/vdl/testdata/base.NestedArgs", nil},
			wiretype.NamedPrimitiveType{1, "anydata", nil},
		}

		if !reflect.DeepEqual(signature.TypeDefs, expectedTypeDefs) {
			t.Errorf("Signature TypeDefs: got %v but expected %v", signature.TypeDefs, expectedTypeDefs)
		}
	}
}
