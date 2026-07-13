package config

import (
	"os"
	"strconv"
	"strings"
)

// envPrefix namespaces the environment variables the config package reads.
const envPrefix = "SLOP_CHOP_"

// envKey maps a flag key like output-dir to SLOP_CHOP_OUTPUT_DIR.
func envKey(key string) string {
	return envPrefix + strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
}

// load resolves key by priority: command-line flag, then environment, then def.
func load(key, def string) string {
	if f, ok := flags[key]; ok && f.Changed {
		return f.Value.String()
	}
	if v, ok := os.LookupEnv(envKey(key)); ok {
		return v
	}
	return def
}

// Set reports whether key took its value from a flag or an environment variable rather than
// falling back to its default, mirroring load's flag-then-env precedence. It lets a caller
// tell an explicit choice from the built-in default even when the two are equal, which
// Changed cannot, since Changed sees only the command-line flag and not the environment.
func Set(key string) bool {
	if f, ok := flags[key]; ok && f.Changed {
		return true
	}
	_, ok := os.LookupEnv(envKey(key))
	return ok
}

// loadString resolves key as a string.
func loadString(key, def string) string {
	return load(key, def)
}

// loadBool resolves key as a bool, falling back to def when the raw value does not
// parse.
func loadBool(key string, def bool) bool {
	b, err := strconv.ParseBool(load(key, strconv.FormatBool(def)))
	if err != nil {
		return def
	}
	return b
}

// loadInt resolves key as an int, falling back to def when the raw value does not parse.
func loadInt(key string, def int) int {
	n, err := strconv.Atoi(load(key, strconv.Itoa(def)))
	if err != nil {
		return def
	}
	return n
}
