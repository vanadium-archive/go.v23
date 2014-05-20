// Package naming defines the public interface for naming, including
// the format of names, the APIs for resolving and managing names as well as
// all associated types.
//
// Veyron names are 'resolved' using a MountTable to obtain a MountedServer
// that RPC method invocations can be directed at. MountTables may be
// mounted on each other to typically create a hierarchy. The name resolution
// process can thus involve multiple MountTables. Although it is expected that
// a hierarchy will be the typical use, it is nonetheless possible to create
// a cyclic graph of MountTables which will lead to name resolution errors
// at runtime.
//
// Veyron names are strings with / used to separate the components of a name.
// Names may be started with / and the address of a MountTable or server,
// in which case they are considerd 'rooted', otherwise they are 'relative' to
// the MountTable used to resolve them. Rooted names, unlike relative ones,
// have the same meaning regardless of the context in which they are accessed.
//
// The first component of a rooted name is the address of the MountTable
// to use for resolving the remainding components of the name. The address
// may be the string representation of a Endpoint, a <host>:<port>,
// or ip>:<port>. In addition, <host> or <ip> maybe used without a <port>
// being specified in which case a default port is used. The portion of the
// name following the address is a relative name.
//
// In addition, //, is treated specially by the implementation of the
// MountTable and is used to instruct the MountTable to stop its resolution
// process at the point where it encounters the //. The portion of the
// name from a '//'' onward is referred to as 'terminal' since it
// terminates the resolution process. An empty name is considered
// terminal. The use of // is primarily intended for implementing the
// MountTable and for debugging since it provides a facility for the user
// to control when resolution stops.
//
// Thus:
//
// /host:port/a/b/c/d means starting at host:port resolve a/b/c/d and
// return the terminating server and the relative path from that server.
//
// /host:port//a/b/c/d means don't bother resolving. Just assume the server is
// at host:port and the object name at the server is a/b/c/d
//
// /host:port/a/b//c/d means resolve up through b.
//

package naming
