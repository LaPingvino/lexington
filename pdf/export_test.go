package pdf

import "time"

// FixDate makes every PDF's dates the same, so that two PDFs of a
// script compare.
func FixDate() { fixedDate = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
