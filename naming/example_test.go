package naming_test

import (
	"fmt"

	"veyron2/naming"
)

func ExampleMakeTerminal() {
	// Create an endpoint string for any tcp port on localhost.
	endPoint := naming.FormatEndpoint("tcp", "localhost:0")

	// Create a name for a service, 'example/foo', served on that endpoint.
	name := naming.JoinAddressName(endPoint, "example/foo")
	fmt.Printf("Name: %q\n", name)

	// Create an endpoint string for a global mounttable
	globalMT := naming.FormatEndpoint("tcp", "v.google.com:8080")

	// Create a name for service, published to the mounttable at
	// point 'users/you' serving names with the prefix
	// 'yourservice'
	nameForYou := naming.JoinAddressName(globalMT, naming.Join("users/you", naming.MakeTerminal("yourservice")))
	sameNameForYou := naming.JoinAddressName(globalMT, "users/you//yourservice")

	fmt.Printf("Name for you: %q\n", nameForYou)
	fmt.Printf("Same name for you: %q\n", sameNameForYou)

	// A terminal name starts with //, MakeTerminal ensures that a name
	// starts with //, wherease MakeResolovable ensures that it does not.

	fmt.Printf("Terminal name: %q\n", naming.MakeTerminal("a/b"))
	fmt.Printf("Resolvable name: %q\n", naming.MakeResolvable("//a/b"))
	fmt.Printf("Resolvable rooted name: %q\n", naming.MakeResolvable("/a/b"))

	// Names can be concatenated taking care to handle / correctly.
	fmt.Printf("Simple join: %q\n", naming.Join("a", "b"))
	fmt.Printf("Rooted join: %q\n", naming.Join("/a", "b"))
	fmt.Printf("Terminal join: %q\n", naming.Join("/a", "//b"))

	// Output:
	// Name: "/@2@tcp@localhost:0@@@@@/example/foo"
	// Name for you: "/@2@tcp@v.google.com:8080@@@@@/users/you//yourservice"
	// Same name for you: "/@2@tcp@v.google.com:8080@@@@@/users/you//yourservice"
	// Terminal name: "//a/b"
	// Resolvable name: "a/b"
	// Resolvable rooted name: "/a/b"
	// Simple join: "a/b"
	// Rooted join: "/a/b"
	// Terminal join: "/a//b"
}
