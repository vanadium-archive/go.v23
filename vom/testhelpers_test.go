package vom

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// equivilentJSON is a helper for tests that checks that json strings are semantically equivalent
// and returns an error if they are not.
func equivalentJSON(j1, j2 string) error {
	var v1, v2 interface{}
	if err := json.Unmarshal([]byte(j1), &v1); err != nil {
		return fmt.Errorf("failed to parse %s: %v", j1, err)
	}
	if err := json.Unmarshal([]byte(j2), &v2); err != nil {
		return fmt.Errorf("failed to parse %s: %v", j2, err)
	}

	if !reflect.DeepEqual(v1, v2) {
		return fmt.Errorf("json objects don't match: '%s' and '%s'", j1, j2)
	}

	return nil
}
