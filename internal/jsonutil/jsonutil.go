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

// OrEmpty returns a non-nil slice so JSON output shows an empty array instead of null.
func OrEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
