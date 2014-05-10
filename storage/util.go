package storage

import (
	"bytes"
	"fmt"
)

func (acl ACL) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "ACL{Name:%q", acl.Name)
	for p, l := range acl.Contents {
		fmt.Fprintf(&buf, ", %s:%s", p, l)
	}
	buf.WriteRune('}')
	return buf.String()
}
