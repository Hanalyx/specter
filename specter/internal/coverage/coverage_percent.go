// coverage_percent.go -- the single definition of a coverage percentage.
//
// Before this, three surfaces computed the number independently and
// disagreed: the table rounded, `explain` truncated by integer division, and
// the stored value truncated to one decimal. On one spec the table printed 67,
// `explain` printed 66, and the gate compared 66.6.
//
// C-35 settles it. One function produces the stored value and one function
// produces the displayed text, and every surface calls them.
//
// Pure functions. No I/O.
//
// @spec spec-coverage
package coverage

import (
	"fmt"
	"math"
)

// CoveragePercent returns covered divided by total as a percentage, rounded
// half up to one decimal place. This is the value emitted as `coverage_pct`
// and the value the threshold gate compares.
//
// Two boundaries matter. A total of zero returns 0 rather than NaN, so an
// empty spec cannot poison a report. And 100.0 is returned only when covered
// equals total: a spec one criterion short of complete rounds to 99.9, not to
// 100.0. Without that cap a spec at 2999 of 3000 would round to 100.0 and pass
// a Tier 1 threshold of 100 while a criterion sits uncovered.
//
// C-35(a).
func CoveragePercent(covered, total int) float64 {
	if total <= 0 {
		return 0
	}
	if covered >= total {
		return 100.0
	}
	pct := float64(covered) / float64(total) * 100
	// math.Round is half away from zero, which is half up for a percentage.
	rounded := math.Round(pct*10) / 10
	if rounded >= 100 {
		return 99.9
	}
	return rounded
}

// FormatCoveragePct renders a stored percentage for a human reader: the
// integer floor plus a percent sign. Every human-readable surface uses it, so
// the number an operator reads is never above the number the gate compared.
//
// Flooring rather than rounding is the deliberate half of C-35. Under
// rounding, 199 of 200 criteria would print 100 and an operator setting a
// threshold of 100 from what they read would be surprised twice.
//
// The floor is exact. CoveragePercent returns a value of the form k/10, and
// when k is a multiple of 10 that division is exact in float64, so a value
// that should read 70 never floors to 69.
//
// C-35(b).
func FormatCoveragePct(pct float64) string {
	return fmt.Sprintf("%d%%", int(math.Floor(pct)))
}
