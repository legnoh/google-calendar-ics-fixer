package main

import (
	"testing"

	ics "github.com/arran4/golang-ical"
)

func TestStableUIDIsDeterministic(t *testing.T) {
	e1 := ics.NewEvent("original-1")
	e1.AddProperty(ics.ComponentPropertySummary, "Weekly Sync")
	e1.AddProperty(ics.ComponentPropertyDtStart, "20260321T100000Z")
	e1.AddProperty(ics.ComponentPropertyDtEnd, "20260321T103000Z")
	e1.AddProperty(ics.ComponentPropertyDescription, "<b>Status</b><br>Updates")
	e1.AddProperty(ics.ComponentPropertyLocation, "Room A")
	e1.AddProperty(ics.ComponentPropertyDtstamp, "20260320T090000Z")

	e2 := ics.NewEvent("original-2")
	e2.AddProperty(ics.ComponentPropertyLocation, "Room B")
	e2.AddProperty(ics.ComponentPropertyDescription, "Status\nUpdates")
	e2.AddProperty(ics.ComponentPropertyDtEnd, "20260321T103000Z")
	e2.AddProperty(ics.ComponentPropertySummary, "Weekly Sync")
	e2.AddProperty(ics.ComponentPropertyDtStart, "20260321T100000Z")
	e2.AddProperty(ics.ComponentPropertyDtstamp, "20260321T090000Z")

	if got, want := stableUID(e1), stableUID(e2); got != want {
		t.Fatalf("stableUID() mismatch: got %q want %q", got, want)
	}
}

func TestDedupeUIDAddsStableSuffixForDuplicates(t *testing.T) {
	seen := map[string]int{}
	base := "abc123@calendar.lkj.io"

	if got := dedupeUID(base, seen); got != base {
		t.Fatalf("first dedupeUID() = %q, want %q", got, base)
	}

	if got := dedupeUID(base, seen); got != "abc123-2@calendar.lkj.io" {
		t.Fatalf("second dedupeUID() = %q", got)
	}
}
