package sanitize

// Band is a coarse reading of a slop score: one of the three ranges the published
// legend names. It exists so every surface that colors or labels a score agrees on
// where the lines fall, rather than each one carrying its own copy of the numbers.
type Band string

// The bands a score falls in.
const (
	// BandLow is a score under 25. The text reads clean.
	BandLow Band = "low"
	// BandMid is a score from 25 to 54. The text is mixed.
	BandMid Band = "mid"
	// BandHigh is a score of 55 or more. The text is dense with tells.
	BandHigh Band = "high"
)

// The band boundaries. A value under bandMidAt is low, under bandHighAt is mid, and
// anything else is high.
const (
	bandMidAt  = 25
	bandHighAt = 55
)

// Band returns the band the score's value falls in.
func (s Score) Band() Band {
	return BandOf(s.Value)
}

// BandOf returns the band a raw 0 to 100 score value falls in. Values outside that
// range are clamped by the comparison rather than rejected, so a caller cannot get an
// empty band back.
func BandOf(value int) Band {
	switch {
	case value < bandMidAt:
		return BandLow
	case value < bandHighAt:
		return BandMid
	default:
		return BandHigh
	}
}
