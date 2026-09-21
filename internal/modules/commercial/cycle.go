package commercial

import "time"

// MonthBoundary moves anchor by months while clamping its day to the target month.
func MonthBoundary(anchor time.Time, months int) time.Time {
	first := time.Date(anchor.Year(), anchor.Month()+time.Month(months), 1,
		anchor.Hour(), anchor.Minute(), 0, 0, anchor.Location())
	last := first.AddDate(0, 1, -1).Day()
	day := anchor.Day()
	if day > last {
		day = last
	}
	return time.Date(first.Year(), first.Month(), day, first.Hour(), first.Minute(), 0, 0, first.Location())
}
