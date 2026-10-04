// Package badge renders a slop score as the flat SVG badge a README embeds. The
// colors and the band boundaries are the ones the published legend names, so a badge
// and the web app read the same number the same way.
//
// The label is fixed. Nothing a caller supplies reaches the markup, which keeps a
// hosted badge endpoint from being turned into a way to put arbitrary text on the
// project's own domain.
package badge

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"

	"github.com/dcadolph/slop-chop/sanitize"
)

// tmplFS holds the badge template.
//
//go:embed badge.svg.tmpl
var tmplFS embed.FS

// tmpl is the parsed badge template, compiled once at startup.
//
//nolint:gochecknoglobals // Parsed once, never modified.
var tmpl = template.Must(template.ParseFS(tmplFS, "badge.svg.tmpl"))

// The band colors, matching the legend on slop-chop.com.
const (
	colorLow     = "#9bcf1a"
	colorMid     = "#e8b93e"
	colorHigh    = "#ff7b72"
	colorUnknown = "#9e9e9e"
)

// label is the left box's text. It never varies.
const label = "slop score"

// Badge geometry. The label box is sized for the fixed label at 11px, and the value
// box grows with the number of characters it has to hold.
const (
	// labelWidth is the left box's width in pixels, sized for the fixed label.
	labelWidth = 70
	// valuePadding is the horizontal padding around the value text.
	valuePadding = 16
	// charWidth is the per-character advance used to size the value box. Digits are
	// uniform width in the fallback fonts, so an estimate is exact for a number.
	charWidth = 7
)

// view is what the template renders. Every field is computed here rather than in the
// template, so the markup holds no arithmetic.
type view struct {
	// Label is the left box's text.
	Label string
	// Value is the right box's text: the score, or a placeholder when unknown.
	Value string
	// Color is the right box's fill.
	Color string
	// Alt is the accessible name, used for both aria-label and the title element.
	Alt string
	// Width is the whole badge's width in pixels.
	Width int
	// LabelWidth is the left box's width in pixels.
	LabelWidth int
	// ValueWidth is the right box's width in pixels.
	ValueWidth int
	// LabelMid is the horizontal center of the left box, where its text anchors.
	LabelMid int
	// ValueMid is the horizontal center of the right box, where its text anchors.
	ValueMid int
}

// SVG renders a badge for one score.
func SVG(score sanitize.Score) (string, error) {
	return render(fmt.Sprintf("%d", score.Value), color(score.Band()),
		fmt.Sprintf("slop score: %d of 100", score.Value))
}

// Unknown renders the gray badge used when the score could not be measured, so an
// embedded image degrades to a readable badge instead of a broken one.
func Unknown() (string, error) {
	return render("n/a", colorUnknown, "slop score: unavailable")
}

// color maps a band to its fill.
func color(b sanitize.Band) string {
	switch b {
	case sanitize.BandLow:
		return colorLow
	case sanitize.BandMid:
		return colorMid
	case sanitize.BandHigh:
		return colorHigh
	default:
		return colorUnknown
	}
}

// render lays out and executes the template for one value.
func render(value, fill, alt string) (string, error) {
	valueWidth := valuePadding + charWidth*len(value)
	v := view{
		Label:      label,
		Value:      value,
		Color:      fill,
		Alt:        alt,
		Width:      labelWidth + valueWidth,
		LabelWidth: labelWidth,
		ValueWidth: valueWidth,
		LabelMid:   labelWidth / 2,
		ValueMid:   labelWidth + valueWidth/2,
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, v); err != nil {
		return "", fmt.Errorf("badge render failed: %w", err)
	}
	return buf.String(), nil
}
