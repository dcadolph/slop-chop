package rewrite

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Verdict is the meaning-comparison result from the judge.
type Verdict struct {
	// Faithful reports whether the rewrite preserved the original's meaning.
	Faithful bool `json:"faithful"`
	// Issues lists each meaning change the judge found.
	Issues []Issue `json:"issues"`
}

// Issue is one meaning change between the original and the rewrite.
type Issue struct {
	// Kind is added, removed, or changed.
	Kind string `json:"kind"`
	// Was is the original wording, empty when the rewrite added something.
	Was string `json:"was"`
	// Now is the rewrite wording, empty when the rewrite removed something.
	Now string `json:"now"`
	// Note is a short reason for the issue.
	Note string `json:"note"`
}

// Judge compares an original text with its rewrite through a Completer and reports any
// change in meaning. It is the optional Layer 3 pass behind the --verify flag.
type Judge struct {
	// completer is the model backend.
	completer Completer
}

// NewJudge returns a Judge. It panics if completer is nil, since that is a programming
// error in this internal package.
func NewJudge(completer Completer) *Judge {
	if completer == nil {
		panic("rewrite.NewJudge: Completer required")
	}
	return &Judge{completer: completer}
}

// Judge asks the model whether rewrite preserves the meaning of original and returns its
// verdict.
func (j *Judge) Judge(ctx context.Context, original, rewrite string) (Verdict, error) {
	user := "ORIGINAL:\n" + original + "\n\nREWRITE:\n" + rewrite
	reply, err := j.completer.Complete(ctx, judgeSystem, user)
	if err != nil {
		return Verdict{}, fmt.Errorf("judge: %w", err)
	}
	obj := jsonObject(reply)
	if obj == "" {
		return Verdict{}, fmt.Errorf("judge: reply held no JSON object")
	}
	var v Verdict
	if err := json.Unmarshal([]byte(obj), &v); err != nil {
		return Verdict{}, fmt.Errorf("judge: decode verdict: %w", err)
	}
	return v, nil
}

// jsonObject returns the substring from the first brace to the last, or empty when the
// text holds no object. It lets the judge tolerate a reply wrapped in code fences or
// stray prose around the JSON.
func jsonObject(text string) string {
	start := strings.IndexByte(text, '{')
	end := strings.LastIndexByte(text, '}')
	if start < 0 || end < start {
		return ""
	}
	return text[start : end+1]
}

// judgeSystem instructs the model to compare meaning and return only a JSON verdict.
const judgeSystem = `You compare an ORIGINAL text with a REWRITE meant to remove AI ` +
	`writing tells while preserving meaning exactly. Report only genuine changes in ` +
	`meaning: facts, numbers, names, claims, logic, or negation that were added, ` +
	`removed, or altered. Ignore wording, style, tone, punctuation, and sentence ` +
	`structure.

Return only a JSON object, with no prose and no code fences:
{"faithful": true, "issues": [{"kind": "changed", "was": "...", "now": "...", "note": "..."}]}

faithful is true when meaning is preserved. kind is "added", "removed", or "changed". ` +
	`For an addition, was is empty; for a removal, now is empty. note is a short reason. ` +
	`When meaning is preserved, return {"faithful": true, "issues": []}.`
