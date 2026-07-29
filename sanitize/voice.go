package sanitize

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"strings"
)

// Voice is a personal style layer laid over a profile: the words and phrases you like kept,
// the swaps you prefer, and the words you want flagged. It maps onto a Profile so the rules
// pass enforces it with no separate machinery.
type Voice struct {
	// Keep lists words and phrases that must never be flagged or swapped, so your signature
	// terms survive even a preset that would cut them. It maps to Profile.Allow.
	Keep []string `json:"keep,omitempty"`
	// Prefer maps a word or phrase to the replacement you want, so a cut lands on your own
	// vocabulary. A single-word key maps to Profile.WordReplace, a multi-word key to
	// Profile.PhraseReplace, and an empty replacement drops the word.
	Prefer map[string]string `json:"prefer,omitempty"`
	// Avoid lists your own words to flag wherever they appear. It maps to Profile.BlockWords,
	// which reports a tell without rewriting it, since a safe replacement depends on context.
	// Use Prefer with an empty replacement to cut a word outright.
	Avoid []string `json:"avoid,omitempty"`
	// Tone holds short notes on how you write, fed to the model rewrite so its output sounds
	// like you. The rules pass ignores it. Write the lines by hand or derive them from your
	// own writing with `voice learn`. It maps to Profile.Tone.
	Tone []string `json:"tone,omitempty"`
}

// LoadVoice reads a Voice from JSON. Any field left unset keeps its zero value, so a partial
// voice is valid.
func LoadVoice(r io.Reader) (Voice, error) {
	var v Voice
	if err := json.NewDecoder(r).Decode(&v); err != nil {
		return Voice{}, fmt.Errorf("voice decode: %w", err)
	}
	return v, nil
}

// LoadVoiceFile reads a Voice from a JSON file at path.
func LoadVoiceFile(path string) (Voice, error) {
	f, err := os.Open(path)
	if err != nil {
		return Voice{}, fmt.Errorf("voice open: %w", err)
	}
	defer func() { _ = f.Close() }()
	return LoadVoice(f)
}

// Empty reports whether the voice sets nothing, so callers can skip applying it and leave a
// profile untouched.
func (v Voice) Empty() bool {
	return len(v.Keep) == 0 && len(v.Prefer) == 0 && len(v.Avoid) == 0 && len(v.Tone) == 0
}

// asProfile turns the voice into a partial profile: keep into Allow, avoid into BlockWords,
// and each prefer entry into WordReplace when its key is one word or PhraseReplace when it is
// several. An empty key is skipped.
func (v Voice) asProfile() Profile {
	p := Profile{Allow: v.Keep, BlockWords: v.Avoid, Tone: v.Tone}
	for from, to := range v.Prefer {
		switch len(strings.Fields(from)) {
		case 0:
			continue
		case 1:
			if p.WordReplace == nil {
				p.WordReplace = make(map[string]string, len(v.Prefer))
			}
			p.WordReplace[from] = to
		default:
			if p.PhraseReplace == nil {
				p.PhraseReplace = make(map[string]string, len(v.Prefer))
			}
			p.PhraseReplace[from] = to
		}
	}
	return p
}

// WithVoice returns p with the voice layered on top. Prefer swaps win over p's own maps, and
// keep and avoid union into Allow and BlockWords. Because compile applies the allow set to
// every rule, a kept word is skipped by every swap and block, so keep beats a preset without
// deleting the preset's entries. An empty voice returns p unchanged.
func (p Profile) WithVoice(v Voice) Profile {
	if v.Empty() {
		return p
	}
	return p.Overlay(v.asProfile())
}

// Overlay returns p with top layered on top: top's map entries win on a shared key and the
// slices take the union. It is the reverse of withPreset, used where a higher-priority layer
// like a voice or a project profile must win over what is already there.
func (p Profile) Overlay(top Profile) Profile {
	p.CharReplace = mergeMapTopWins(p.CharReplace, top.CharReplace)
	p.PhraseReplace = mergeMapTopWins(p.PhraseReplace, top.PhraseReplace)
	p.WordReplace = mergeMapTopWins(p.WordReplace, top.WordReplace)
	p.RegexReplace = mergeMapTopWins(p.RegexReplace, top.RegexReplace)
	p.FlagPatterns = mergeMapTopWins(p.FlagPatterns, top.FlagPatterns)
	p.BlockWords = mergeSlice(p.BlockWords, top.BlockWords)
	p.Allow = mergeSlice(p.Allow, top.Allow)
	p.Tone = mergeSlice(p.Tone, top.Tone)
	return p
}

// mergeMapTopWins returns the union of base and top with top winning on a shared key. It
// returns base unchanged when top is empty.
func mergeMapTopWins(base, top map[string]string) map[string]string {
	if len(top) == 0 {
		return base
	}
	out := make(map[string]string, len(base)+len(top))
	maps.Copy(out, base)
	maps.Copy(out, top)
	return out
}
