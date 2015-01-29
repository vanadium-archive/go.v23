package main_test

// This test assumes the vdl packages under veyron2/vdl/testdata have been
// compiled using the vdl binary, and runs end-to-end ipc tests against the
// generated output.  It's meant as a final sanity check of the vdl compiler; by
// using the compiled results we're behaving as an end-user would behave.

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"

	"v.io/core/veyron2"
	"v.io/core/veyron2/context"
	"v.io/core/veyron2/ipc"
	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/vdl/testdata/arith"
	"v.io/core/veyron2/vdl/testdata/base"

	"v.io/core/veyron/lib/testutil"
	_ "v.io/core/veyron/profiles"
)

var generatedError = errors.New("generated error")

func newServer(ctx *context.T) ipc.Server {
	s, err := veyron2.NewServer(ctx)
	if err != nil {
		panic(err)
	}
	return s
}

// serverArith implements the arith.Arith interface.
type serverArith struct{}

var numNoArgsCalls int

func (*serverArith) NoArgs(ipc.ServerContext) {
	numNoArgsCalls++
}

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

func (*serverArith) Count(ctx arith.ArithCountContext, start int32) error {
	const kNum = 1000
	for i := int32(0); i < kNum; i++ {
		if err := ctx.SendStream().Send(start + i); err != nil {
			return err
		}
	}
	return nil
}

func (*serverArith) StreamingAdd(ctx arith.ArithStreamingAddContext) (int32, error) {
	var total int32
	for ctx.RecvStream().Advance() {
		value := ctx.RecvStream().Value()
		total += value
		ctx.SendStream().Send(total)
	}
	return total, ctx.RecvStream().Err()
}

func (*serverArith) GenError(_ ipc.ServerContext) error {
	return generatedError
}

func (*serverArith) QuoteAny(_ ipc.ServerContext, any vdl.AnyRep) (vdl.AnyRep, error) {
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
	ctx, shutdown := testutil.InitForTest()
	defer shutdown()

	server := newServer(ctx)
	eps, err := server.Listen(veyron2.GetListenSpec(ctx))
	if err := server.Serve("", arith.CalculatorServer(&serverCalculator{}), nil); err != nil {
		t.Fatal(err)
	}
	root := eps[0].Name()
	// Synchronous calls
	calculator := arith.CalculatorClient(root)
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

	ar := arith.ArithClient(root)
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

	trig := arith.TrigonometryClient(root)
	cosine, err = trig.Cosine(ctx, 0)
	if err != nil {
		t.Errorf("Cosine: got %q but expected no error", err)
	}
	if cosine != 1 {
		t.Errorf("Cosine: expected 1 got %f", cosine)
	}

	// Test auto-generated methods.
	serverStub := arith.CalculatorServer(&serverCalculator{})
	expectDesc(t, serverStub.Describe__(), []ipc.InterfaceDesc{
		{
			Name:    "Calculator",
			PkgPath: "v.io/core/veyron2/vdl/testdata/arith",
			Embeds: []ipc.EmbedDesc{
				{
					Name:    "Arith",
					PkgPath: "v.io/core/veyron2/vdl/testdata/arith",
				},
				{
					Name:    "AdvancedMath",
					PkgPath: "v.io/core/veyron2/vdl/testdata/arith",
				},
			},
			Methods: []ipc.MethodDesc{
				{Name: "On", OutArgs: []ipc.ArgDesc{{}}},
				{Name: "Off", OutArgs: []ipc.ArgDesc{{}}, Tags: []vdl.AnyRep{"offtag"}},
			},
		},
		{
			Name:    "Arith",
			PkgPath: "v.io/core/veyron2/vdl/testdata/arith",
			Methods: []ipc.MethodDesc{
				{
					Name:    "Add",
					InArgs:  []ipc.ArgDesc{{Name: "a"}, {Name: "b"}},
					OutArgs: []ipc.ArgDesc{{}, {}},
				},
				{
					Name:    "DivMod",
					InArgs:  []ipc.ArgDesc{{Name: "a"}, {Name: "b"}},
					OutArgs: []ipc.ArgDesc{{Name: "quot"}, {Name: "rem"}, {Name: "err"}},
				},
				{
					Name:    "Sub",
					InArgs:  []ipc.ArgDesc{{Name: "args"}},
					OutArgs: []ipc.ArgDesc{{}, {}},
				},
				{
					Name:    "Mul",
					InArgs:  []ipc.ArgDesc{{Name: "nested"}},
					OutArgs: []ipc.ArgDesc{{}, {}},
				},
				{
					Name:    "GenError",
					OutArgs: []ipc.ArgDesc{{}},
					Tags:    []vdl.AnyRep{"foo", "barz", "hello", int32(129), uint64(0x24)},
				},
				{
					Name:    "Count",
					InArgs:  []ipc.ArgDesc{{Name: "start"}},
					OutArgs: []ipc.ArgDesc{{}},
				},
				{
					Name:    "StreamingAdd",
					OutArgs: []ipc.ArgDesc{{Name: "total"}, {Name: "err"}},
				},
				{
					Name:    "QuoteAny",
					InArgs:  []ipc.ArgDesc{{Name: "a"}},
					OutArgs: []ipc.ArgDesc{{}, {}},
				},
			},
		},
		{
			Name:    "AdvancedMath",
			PkgPath: "v.io/core/veyron2/vdl/testdata/arith",
			Embeds: []ipc.EmbedDesc{
				{
					Name:    "Trigonometry",
					PkgPath: "v.io/core/veyron2/vdl/testdata/arith",
				},
				{
					Name:    "Exp",
					PkgPath: "v.io/core/veyron2/vdl/testdata/arith/exp",
				}},
		},
		{
			Name:    "Trigonometry",
			PkgPath: "v.io/core/veyron2/vdl/testdata/arith",
			Doc:     "// Trigonometry is an interface that specifies a couple trigonometric functions.",
			Methods: []ipc.MethodDesc{
				{
					Name: "Sine",
					InArgs: []ipc.ArgDesc{
						{"angle", ``}, // float64
					},
					OutArgs: []ipc.ArgDesc{
						{"", ``}, // float64
						{"", ``}, // error
					},
				},
				{
					Name: "Cosine",
					InArgs: []ipc.ArgDesc{
						{"angle", ``}, // float64
					},
					OutArgs: []ipc.ArgDesc{
						{"", ``}, // float64
						{"", ``}, // error
					},
				},
			},
		},
		{
			Name:    "Exp",
			PkgPath: "v.io/core/veyron2/vdl/testdata/arith/exp",
			Methods: []ipc.MethodDesc{
				{
					Name: "Exp",
					InArgs: []ipc.ArgDesc{
						{"x", ``}, // float64
					},
					OutArgs: []ipc.ArgDesc{
						{"", ``}, // float64
						{"", ``}, // error
					},
				},
			},
		},
	})
}

func TestArith(t *testing.T) {
	ctx, shutdown := testutil.InitForTest()
	defer shutdown()

	// TODO(bprosnitz) Split this test up -- it is quite long and hard to debug.

	// We try a few types of dispatchers on the server side, to verify that
	// anything dispatching to Arith or an interface embedding Arith (like
	// Calculator) works for a client looking to talk to an Arith service.
	objects := []interface{}{
		arith.ArithServer(&serverArith{}),
		arith.ArithServer(&serverCalculator{}),
		arith.CalculatorServer(&serverCalculator{}),
	}

	for i, obj := range objects {
		server := newServer(ctx)
		defer server.Stop()
		eps, err := server.Listen(veyron2.GetListenSpec(ctx))
		if err != nil {
			t.Fatal(err)
		}
		root := eps[0].Name()
		if err := server.Serve("", obj, nil); err != nil {
			t.Fatalf("%d: %v", i, err)
		}
		// Synchronous calls
		ar := arith.ArithClient(root)
		/*
			      // TODO(toddw): Re-enable this when supported by java.
						oldCalls := numNoArgsCalls
						if err := ar.NoArgs(ctx); err != nil {
							t.Errorf("NoArgs: got %q but expected no error", err)
						}
						if got, want := numNoArgsCalls - oldCalls, 1; got != want {
							t.Errorf("NoArgs calls: got %d, want %d", got, want)
						}
		*/
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

		// Server-side stubs

		serverStub := arith.ArithServer(&serverArith{})
		expectDesc(t, serverStub.Describe__(), []ipc.InterfaceDesc{
			{
				Name:    "Arith",
				PkgPath: "v.io/core/veyron2/vdl/testdata/arith",
				Methods: []ipc.MethodDesc{
					{
						Name:    "Add",
						InArgs:  []ipc.ArgDesc{{Name: "a"}, {Name: "b"}},
						OutArgs: []ipc.ArgDesc{{}, {}},
					},
					{
						Name:    "DivMod",
						InArgs:  []ipc.ArgDesc{{Name: "a"}, {Name: "b"}},
						OutArgs: []ipc.ArgDesc{{Name: "quot"}, {Name: "rem"}, {Name: "err"}},
					},
					{
						Name:    "Sub",
						InArgs:  []ipc.ArgDesc{{Name: "args"}},
						OutArgs: []ipc.ArgDesc{{}, {}},
					},
					{
						Name:    "Mul",
						InArgs:  []ipc.ArgDesc{{Name: "nested"}},
						OutArgs: []ipc.ArgDesc{{}, {}},
					},
					{
						Name:    "GenError",
						OutArgs: []ipc.ArgDesc{{}},
						Tags:    []vdl.AnyRep{"foo", "barz", "hello", int32(129), uint64(0x24)},
					},
					{
						Name:    "Count",
						InArgs:  []ipc.ArgDesc{{Name: "start"}},
						OutArgs: []ipc.ArgDesc{{}},
					},
					{
						Name:    "StreamingAdd",
						OutArgs: []ipc.ArgDesc{{Name: "total"}, {Name: "err"}},
					},
					{
						Name:    "QuoteAny",
						InArgs:  []ipc.ArgDesc{{Name: "a"}},
						OutArgs: []ipc.ArgDesc{{}, {}},
					},
				},
			},
		})
	}
}

func expectDesc(t *testing.T, got, want []ipc.InterfaceDesc) {
	stripDesc(got)
	stripDesc(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Describe__ got %#v, want %#v", got, want)
	}
}

func stripDesc(desc []ipc.InterfaceDesc) {
	// Don't bother testing the documentation, to avoid spurious changes.
	for i := range desc {
		desc[i].Doc = ""
		for j := range desc[i].Embeds {
			desc[i].Embeds[j].Doc = ""
		}
		for j := range desc[i].Methods {
			desc[i].Methods[j].Doc = ""
			for k := range desc[i].Methods[j].InArgs {
				desc[i].Methods[j].InArgs[k].Doc = ""
			}
			for k := range desc[i].Methods[j].OutArgs {
				desc[i].Methods[j].OutArgs[k].Doc = ""
			}
			desc[i].Methods[j].InStream.Doc = ""
			desc[i].Methods[j].OutStream.Doc = ""
		}
	}
}
