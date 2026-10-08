package commercial

import (
	"testing"
	"time"
)

func TestMonthBoundaryRestoresAnchor(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*3600)
	anchor := time.Date(2028, 1, 31, 10, 0, 0, 0, loc)
	if got := MonthBoundary(anchor, 1); got.Day() != 29 {
		t.Fatal(got)
	}
	if got := MonthBoundary(anchor, 2); got.Day() != 31 {
		t.Fatal(got)
	}
}

func TestMonthBoundaryPreservesAnchorLocationAndTime(t *testing.T) {
	loc := time.FixedZone("display-zone", -5*3600)
	anchor := time.Date(2027, 3, 30, 23, 59, 42, 999, loc)
	got := MonthBoundary(anchor, -2)
	if got.Format("2006-01-02 15:04:05 -0700 MST") != "2027-01-30 23:59:00 -0500 display-zone" {
		t.Fatalf("got %v", got)
	}
}
