// Package naming defines the public interface for naming, including
// the format of names, the APIs for resolving and managing names as well as
// all associated types.
//
// Veyron names are 'resolved' using a MountTable to obtain a MountedServer
// that RPC method invocations can be directed at.
//
// Veyron names are strings with / used to separate the components of a name.
// Names may be start with /, in which case they are considerd 'absolute',
// otherwise they are 'relative'. Absolute names have the same meaning
// regardless of the context in which they are accessed, relative names are
// only meaningful within the context of the a given process.
//
// The first component of an absolute name is the address of the MountTable
// to use for resolving the remainding components of the name. The address
// may be the string representation of a Endpoint, a <host>:<port> or
// <ip>:<port>.
//
// In addition, //, is treated specially by the implementation of the
// MountTable and instructs it to stop its resolution process at the
// point where it encounters the //. It is an error to include // more than
// once in a name. The use of // is optional.
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
