package stream_test

import (
	"fmt"
	"os"
)

func Example_EchoServer() {
	m, _ := RT.NewStreamManager()
	ln, ep, _ := m.Listen("127.0.0.0", "0")
	fmt.Println("Listening on:", ep)
	buf := make([]byte, 1024)
	for {
		flow, _ := ln.Accept()
		for {
			n, err := flow.Read(buf)
			if err != nil {
				break
			}
			flow.Write(buf[:n])
		}
	}
}

func Example_EchoClient() {
	m, _ := RT.NewStreamManager()
	var epStr string
	fmt.Fscanf(os.Stdin, "%s", epStr)
	ep, err := RT.NewEndpoint(epStr)
	if err != nil {
		fmt.Printf("ERROR: Bad endpoint string(%q): %v", epStr, err)
		return
	}
	vc, _ := m.Dial(ep)
	flow, _ := vc.Connect()
	flow.Write([]byte("hello world"))
	flow.Close()
}
