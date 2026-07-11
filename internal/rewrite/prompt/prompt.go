// Package prompt builds the instruction the rewrite pass sends the model. It lives
// apart from the rewrite package so the WASM build can share the exact prompt the CLI
// sends without pulling the HTTP client into the binary.
package prompt

import "strings"

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
