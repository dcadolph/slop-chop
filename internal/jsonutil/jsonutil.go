// Package jsonutil centralizes JSON encoding so output is compact by default and
// indented only when asked.
package jsonutil

import "encoding/json"

// Marshal encodes v as JSON. When pretty is true the output is indented with two
// spaces, otherwise it is compact.
func Marshal(v any, pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(v, "", "  ")
	}
	return json.Marshal(v)
}
