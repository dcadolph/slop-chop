package sanitize

import "strings"

// whyReasons says in plain words why each class of rule fires. The web app carries the
// same sentences in its findings panel, so a finding reads the same on every surface.
//
//nolint:gochecknoglobals // Immutable lookup.
var whyReasons = map[string]string{
	"word":       "This word sits on the block list of terms models lean on.",
	"phrase":     "A stock phrase of the machine register.",
	"char":       "A character the profile normalizes. Only the em-dash and invisible characters count toward the score.",
	"structural": "A sentence shape of model prose. Shapes count double toward the score.",
	"replace":    "A word swap from the profile or a preset.",
	"drop":       "A word the profile or a preset cuts outright.",
	"case":       "Your voice fixes how this word is capitalized.",
	"regex":      "One of your own regex rules.",
	"spelling":   "The dialect setting enforces this spelling.",
}

// Why says in plain words why the finding fired and what happens to it, so a reader never
// has to decode a rule name. The class carries the reason and the replacement carries the
// action.
func Why(f Finding) string {
	cls := f.Rule
	if c, _, ok := strings.Cut(f.Rule, ":"); ok {
		cls = c
	}
	reason, ok := whyReasons[cls]
	if !ok {
		reason = "House-style cleanup. It carries no score weight."
	}
	switch {
	case f.Replacement == nil:
		return reason + " It only flags; the rewording is your call."
	case *f.Replacement == "":
		return reason + " The profile cuts it."
	}
	return reason + " The profile swaps it."
}
