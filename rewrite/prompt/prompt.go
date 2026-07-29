// Package prompt builds the instructions the rewrite and verify passes send the model.
// It lives apart from the rewrite package so the WASM build can share the exact prompts
// the CLI sends without pulling the HTTP client into the binary.
package prompt

import "strings"

// Judge returns the instruction that tells the model to compare an original text with
// its rewrite and report meaning changes as a JSON verdict.
func Judge() string {
	return judgeSystem
}

// Learn returns the instruction that tells the model to study writing samples and describe
// the author's voice as short tone notes, the lines the rewrite pass matches against.
func Learn() string {
	return learnSystem
}

// learnSystem instructs the model to derive tone notes and return only a JSON array.
const learnSystem = `You study samples of one author's writing and describe their voice, so ` +
	`another pass can rewrite text to sound like them. Focus on what is distinctive and ` +
	`reusable: sentence length and rhythm, formality, contractions, humor, directness, ` +
	`favorite constructions, how they open and close. Ignore the subject matter.

Return only a JSON array of 3 to 6 short strings, each one tone note under 12 words, ` +
	`no prose and no code fences. Example:
["short, blunt sentences", "dry humor, no hype", "contractions everywhere", "opens with the point"]`

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

// System assembles the instruction that tells the model how to clean the text. Tone
// notes shape the voice to aim for. Feedback notes name facts a prior attempt changed,
// so a retry keeps them; pass none on a first attempt.
func System(tone, feedback []string) string {
	var b strings.Builder
	b.WriteString("You rewrite text so it reads like a person wrote it, not a chatbot. ")
	b.WriteString("Keep the meaning and the facts unchanged. Do not add or remove ideas.\n\n")
	b.WriteString("Remove the tells of AI writing:\n")
	b.WriteString("- No em-dashes. Recast the sentence or use a comma or a period.\n")
	b.WriteString("- No semicolons joining clauses. Split them into separate sentences.\n")
	b.WriteString("- Drop filler openers like \"In summary\" and \"To be honest\".\n")
	b.WriteString("- Cut buzzwords like \"comprehensive\" and \"robust\".\n")
	b.WriteString("- Vary sentence length. Avoid the flat, even cadence models fall into.\n")
	b.WriteString("- Use plain words and contractions where they fit.\n\n")
	if len(tone) > 0 {
		b.WriteString("Match this voice:\n")
		for _, t := range tone {
			b.WriteString("- ")
			b.WriteString(t)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	if len(feedback) > 0 {
		b.WriteString("A prior rewrite changed the meaning. Keep these facts exactly this time:\n")
		for _, note := range feedback {
			b.WriteString("- ")
			b.WriteString(note)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("Return only the rewritten text. No preamble, no quotes, no notes.")
	return b.String()
}
