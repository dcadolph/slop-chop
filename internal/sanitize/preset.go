package sanitize

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strings"
)

// presetFS holds the built-in preset profiles.
//
//go:embed presets/*.json
var presetFS embed.FS

// ApplyPresets merges the named built-in presets into p and returns the result. A preset
// only fills entries p does not already set, so an explicit profile always wins over a
// preset. An unknown name is an error that lists the presets that exist.
func ApplyPresets(p Profile, names ...string) (Profile, error) {
	for _, name := range names {
		pack, err := loadPreset(name)
		if err != nil {
			return Profile{}, err
		}
		p = p.withPreset(pack)
	}
	return p, nil
}

// loadPreset returns the built-in preset profile by name.
func loadPreset(name string) (Profile, error) {
	data, err := presetFS.ReadFile("presets/" + name + ".json")
	if err != nil {
		return Profile{}, fmt.Errorf("%w: %q: have %s", ErrPreset, name, strings.Join(PresetNames(), ", "))
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return Profile{}, fmt.Errorf("%w: %q: %w", ErrPreset, name, err)
	}
	return p, nil
}

// PresetNames lists the built-in preset names, sorted.
func PresetNames() []string {
	entries, err := fs.ReadDir(presetFS, "presets")
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, strings.TrimSuffix(e.Name(), ".json"))
	}
	slices.Sort(names)
	return names
}

// withPreset returns p with pack merged in. Maps take pack's entries where p has none, and
// slices take the union, so p always wins on a conflict. Booleans, tone, and dialect stay
// as p has them, since a preset adds rules rather than forcing settings.
func (p Profile) withPreset(pack Profile) Profile {
	p.CharReplace = mergeMap(p.CharReplace, pack.CharReplace)
	p.PhraseReplace = mergeMap(p.PhraseReplace, pack.PhraseReplace)
	p.WordReplace = mergeMap(p.WordReplace, pack.WordReplace)
	p.RegexReplace = mergeMap(p.RegexReplace, pack.RegexReplace)
	p.BlockWords = mergeSlice(p.BlockWords, pack.BlockWords)
	p.Allow = mergeSlice(p.Allow, pack.Allow)
	return p
}

// mergeMap returns the union of base and add with base winning on a shared key. It returns
// base unchanged when add is empty.
func mergeMap(base, add map[string]string) map[string]string {
	if len(add) == 0 {
		return base
	}
	out := make(map[string]string, len(base)+len(add))
	maps.Copy(out, add)
	maps.Copy(out, base)
	return out
}

// mergeSlice returns the union of base and add in order, dropping duplicates. It returns
// base unchanged when add is empty.
func mergeSlice(base, add []string) []string {
	if len(add) == 0 {
		return base
	}
	seen := make(map[string]bool, len(base)+len(add))
	out := make([]string, 0, len(base)+len(add))
	for _, v := range slices.Concat(base, add) {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
