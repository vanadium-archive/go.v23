package config_test

import (
	"fmt"
	"math/rand"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"veyron2/config"
)

func ExamplePublisher() {
	in := make(chan config.Setting)
	pub := config.NewPublisher()
	pub.CreateStream("net", "network settings", in)

	// A simple producer of IP address settings.
	producer := func() {
		in <- config.NewString("ip", "address", "1.2.3.5")
	}

	var waiter sync.WaitGroup
	waiter.Add(2)

	// A simple consumer of IP address Settings.
	consumer := func(ch chan config.Setting) {
		fmt.Println(<-ch)
		waiter.Done()
	}

	// Publish an initial Setting to the Stream.
	in <- config.NewString("ip", "address", "1.2.3.4")

	// Fork the stream twice, and read the latest value.
	ch1 := make(chan config.Setting)
	st, _ := pub.ForkStream("net", ch1)
	fmt.Println(st.Latest["ip"])
	ch2 := make(chan config.Setting)
	st, _ = pub.ForkStream("net", ch2)
	fmt.Println(st.Latest["ip"])

	// Now we can read new Settings as they are generated.
	go producer()
	go consumer(ch1)
	go consumer(ch2)

	waiter.Wait()

	// Output:
	// ip: address: (string: 1.2.3.4)
	// ip: address: (string: 1.2.3.4)
	// ip: address: (string: 1.2.3.5)
	// ip: address: (string: 1.2.3.5)
}

func ExampleShutdown() {
	in := make(chan config.Setting)
	pub := config.NewPublisher()
	stop, _ := pub.CreateStream("net", "network settings", in)

	var ready sync.WaitGroup
	ready.Add(1)

	// A producer to write 100 Settings before signalling that it's
	// ready to be shutdown. This is purely to demonstrate how to use
	// Shutdown.
	producer := func() {
		for i := 0; ; i++ {
			select {
			case <-stop:
				close(in)
				return
			default:
				in <- config.NewString("ip", "address", "1.2.3.4")
				if i == 100 {
					ready.Done()
				}
			}
		}
	}

	var waiter sync.WaitGroup
	waiter.Add(2)

	consumer := func() {
		ch := make(chan config.Setting, 10)
		pub.ForkStream("net", ch)
		i := 0
		for {
			if _, ok := <-ch; !ok {
				// The channel has been closed when the publisher
				// is asked to shut down.
				break
			}
			i++
		}
		if i >= 100 {
			// We've received at least 100 Settings as per the producer above.
			fmt.Println("done")
		}
		waiter.Done()
	}

	go producer()
	go consumer()
	go consumer()
	ready.Wait()
	pub.Shutdown()
	waiter.Wait()
	// Output:
	// done
	// done
}

func TestSimple(t *testing.T) {
	ch := make(chan config.Setting, 2)
	pub := config.NewPublisher()
	if _, err := pub.ForkStream("stream", nil); err == nil || err.Error() != "stream \"stream\" doesn't exist" {
		t.Errorf("missing or wrong error: %v", err)
	}
	if _, err := pub.CreateStream("stream", "example", nil); err == nil || err.Error() != "must provide a non-nil channel" {
		t.Fatalf("missing or wrong error: %v", err)
	}
	if _, err := pub.CreateStream("stream", "example", ch); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if _, err := pub.CreateStream("stream", "example", ch); err == nil || err.Error() != "stream \"stream\" already exists" {
		t.Fatalf("missing or wrong error: %v", err)
	}
	if got, want := pub.String(), "(stream: example)"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	stop, err := pub.CreateStream("s2", "eg2", ch)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if got, want := pub.String(), "(stream: example) (s2: eg2)"; got != want {
		wantAlternate := "(s2: eg2) (stream: example)"
		if got != wantAlternate {
			t.Errorf("got %q, want %q or %q", got, want, wantAlternate)
		}
	}

	got, want := pub.Latest("s2"), &config.Stream{"s2", "eg2", nil}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	pub.Shutdown()
	if _, running := <-stop; running {
		t.Errorf("expected to be shutting down")
	}
	if _, err := pub.ForkStream("stream", nil); err == nil || err.Error() != "stream \"stream\" has been shut down" {
		t.Errorf("missing or wrong error: %v", err)
	}
	if got, want := pub.String(), "shutdown"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}

}

func producer(pub *config.Publisher, in chan<- config.Setting, stop <-chan struct{}, limit int, ch chan int, waiter *sync.WaitGroup) {
	for i := 0; ; i++ {
		select {
		case <-stop:
			ch <- i
			waiter.Done()
			// must close this channel, otherwise the Publisher will leak a goroutine for this stream.
			close(in)
			return
		default:
			// signal progress on ch, at limit/2, limit and when we're done (above)
			switch {
			case i == limit/2:
				ch <- i
			case i == limit:
				ch <- i
			}
			if i%2 == 0 {
				in <- config.NewInt("i", "int", i)
			} else {
				in <- config.NewFloat64("f", "float", float64(i))
			}
		}
	}
	panic("should never get here")
}

func consumer(t *testing.T, pub *config.Publisher, limit, bufsize int, waiter *sync.WaitGroup) {
	ch := make(chan config.Setting, bufsize)
	st, _ := pub.ForkStream("net", ch)
	i, i2 := 0, 0
	if st.Latest["i"] != nil {
		i = int(st.Latest["i"].Value().(int))
	}
	if st.Latest["f"] != nil {
		i2 = int(st.Latest["f"].Value().(float64))
	}
	if i2 > i {
		i = i2
	}
	i++
	for s := range ch {
		switch v := s.Value().(type) {
		case int:
			if i%2 != 0 {
				t.Errorf("expected a float, got an int")
				break
			}
			if v != i {
				t.Errorf("got %d, want %d", v, i)
			}
		case float64:
			if i%2 != 1 {
				t.Errorf("expected an int, got a float")
				break
			}
			if v != float64(i) {
				t.Errorf("got %f, want %f", v, i)
			}
		}
		i++
	}
	if i < limit {
		t.Errorf("didn't read enough settings: got %d, want >= %d", i, limit)
	}
	waiter.Done()
}

func testStream(t *testing.T, consumerBufSize int) {
	in := make(chan config.Setting)
	pub := config.NewPublisher()
	stop, _ := pub.CreateStream("net", "network settings", in)

	rand.Seed(time.Now().UnixNano())
	limit := rand.Intn(5000)
	if limit < 100 {
		limit = 100
	}
	t.Logf("limit: %d", limit)

	var waiter sync.WaitGroup
	waiter.Add(3)

	progress := make(chan int)
	go producer(pub, in, stop, limit, progress, &waiter)

	i := <-progress
	t.Logf("limit/2 = %d", i)

	// We use a lot of buffering in this unittest to ensure that
	// we never miss any settings.
	go consumer(t, pub, limit, consumerBufSize, &waiter)
	go consumer(t, pub, limit, consumerBufSize, &waiter)

	reached := <-progress
	pub.Shutdown()
	shutdown := <-progress
	t.Logf("reached %d, shut down at %d", reached, shutdown)

	// Wait for all goroutines to finish.
	waiter.Wait()
}

func TestStream(t *testing.T) {
	testStream(t, 500)
}

func TestStreamSmallBuffers(t *testing.T) {
	testStream(t, 1)
}

func TestDurationFlag(t *testing.T) {
	d := &config.DurationFlag{}
	if err := d.Set("1s"); err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if got, want := d.Duration, time.Duration(time.Second); got != want {
		t.Errorf("got %s, expected %s", got, want)
	}
	if err := d.Set("1t"); err == nil || err.Error() != "time: unknown unit t in duration 1t" {
		t.Errorf("expected error %v", err)
	}
}

func TestIPFlag(t *testing.T) {
	ip := &config.IPFlag{}
	if err := ip.Set("172.16.1.22"); err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if got, want := ip.IP, net.ParseIP("172.16.1.22"); !got.Equal(want) {
		t.Errorf("got %s, expected %s", got, want)
	}
	if err := ip.Set("172.16"); err == nil || err.Error() != "failed to parse 172.16 as an IP address" {
		t.Errorf("expected error %v", err)
	}
}

func TestTCPFlag(t *testing.T) {
	tcp := &config.TCPProtocolFlag{}
	if err := tcp.Set("tcp6"); err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if got, want := tcp.Protocol, "tcp6"; got != want {
		t.Errorf("got %s, expected %s", got, want)
	}
	if err := tcp.Set("foo"); err == nil || !strings.Contains(err.Error(), "not a tcp protocol") {
		t.Errorf("expected error %v", err)
	}
}

func TestIPHostPortFlag(t *testing.T) {
	lh := []net.IP{net.ParseIP("127.0.0.1")}
	ip6 := []net.IP{net.ParseIP("FE80:0000:0000:0000:0202:B3FF:FE1E:8329")}
	cases := []struct {
		input string
		want  config.IPHostPortFlag
		str   string
	}{
		{"", config.IPHostPortFlag{Port: "0"}, ":0"},
		{":0", config.IPHostPortFlag{Port: "0"}, ":0"},
		{":22", config.IPHostPortFlag{Port: "22"}, ":22"},
		{"127.0.0.1", config.IPHostPortFlag{IP: lh, Port: "0"}, "127.0.0.1:0"},
		{"127.0.0.1:10", config.IPHostPortFlag{IP: lh, Port: "10"}, "127.0.0.1:10"},
		{"[]:0", config.IPHostPortFlag{Port: "0"}, ":0"},
		{"[FE80:0000:0000:0000:0202:B3FF:FE1E:8329]:100", config.IPHostPortFlag{IP: ip6, Port: "100"}, "[fe80::202:b3ff:fe1e:8329]:100"},
	}
	for _, c := range cases {
		got, want := &config.IPHostPortFlag{}, &c.want
		if err := got.Set(c.input); err != nil || !reflect.DeepEqual(got, want) {
			if err != nil {
				t.Errorf("%q: unexpected error %s", c.input, err)
			} else {
				t.Errorf("%q: got %#v, want %#v", c.input, got, want)
			}
		}
		if got.String() != c.str {
			t.Errorf("%q: got %#v, want %#v", c.input, got.String(), c.str)
		}
	}

	host := &config.IPHostPortFlag{}
	if err := host.Set("localhost:122"); err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if len(host.IP) == 0 {
		t.Errorf("localhost should have resolved to at least one address")
	}
	if got, want := host.Port, "122"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := host.String(), "localhost:122"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	for _, s := range []string{
		":", ":59999999", "__nohost__", "__nohost__:"} {
		f := &config.IPHostPortFlag{}
		if err := f.Set(s); err == nil {
			t.Errorf("expected an error for %q", s)
		}
	}

}
