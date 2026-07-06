package config

import "strconv"

// FlagValue is a pflag.Value backed by a raw string. Every scalar flag in the config
// package uses it so flag state lives in one shape.
type FlagValue struct {
	// Val is the current raw value.
	Val string
	// ValType is the type name pflag shows in help output, like string or bool.
	ValType string
}

// String returns the current raw value.
func (v *FlagValue) String() string { return v.Val }

// Set validates s against the value type and stores it.
func (v *FlagValue) Set(s string) error {
	if v.ValType == "bool" {
		if _, err := strconv.ParseBool(s); err != nil {
			return err
		}
	}
	v.Val = s
	return nil
}

// Type returns the pflag type name.
func (v *FlagValue) Type() string { return v.ValType }
