package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
)

var (
	reBR  = regexp.MustCompile(`(?i)<br\s*/?>`)
	reTag = regexp.MustCompile(`<[^>]+>`)
)

func newUID(index int) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d-%d@calendar.lkj.io", time.Now().UnixNano(), index)
	}
	return fmt.Sprintf("%d-%s-%d@calendar.lkj.io", time.Now().UnixNano(), hex.EncodeToString(b), index)
}

func stripHTMLish(s string) string {
	// ICS TEXTでは "\n" は "\n" のままでも大抵大丈夫（Serializeで折返しされる）
	s = html.UnescapeString(s)
	s = reBR.ReplaceAllString(s, "\n")
	s = reTag.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

func getPropValue(e *ics.VEvent, prop ics.ComponentProperty) (string, bool) {
	p := e.GetProperty(prop)
	if p == nil {
		return "", false
	}
	return p.Value, true
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "Usage: google-calendar-ics-fixer <input.ics> <output.ics>")
		os.Exit(2)
	}
	inPath := os.Args[1]
	outPath := os.Args[2]

	in, err := os.ReadFile(inPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		os.Exit(1)
	}

	cal, err := ics.ParseCalendar(bytes.NewReader(in))
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse:", err)
		os.Exit(1)
	}

	events := cal.Events()
	recurrenceProps := []ics.ComponentProperty{
		ics.ComponentPropertyRrule,
		ics.ComponentPropertyRdate,
		ics.ComponentPropertyExrule,
		ics.ComponentPropertyExdate,
		ics.ComponentPropertyRecurrenceId,
	}

	recurrenceRemoved := 0
	for i, e := range events {
		// UID は必ず新規採番
		e.ReplaceProperty(ics.ComponentPropertyUniqueId, newUID(i))

		// RRULE など繰り返し関連プロパティは無視（削除）
		for _, prop := range recurrenceProps {
			recurrenceRemoved += len(e.RemoveProperty(prop))
		}

		// DESCRIPTION / LOCATION を軽くサニタイズ
		if v, ok := getPropValue(e, ics.ComponentPropertyDescription); ok && strings.Contains(v, "<") {
			e.ReplaceProperty(ics.ComponentPropertyDescription, stripHTMLish(v))
		}
		if v, ok := getPropValue(e, ics.ComponentPropertyLocation); ok && strings.Contains(v, "<") {
			e.ReplaceProperty(ics.ComponentPropertyLocation, stripHTMLish(v))
		}
	}

	// シリアライズ（CRLF + 75折返し）
	out := cal.Serialize(
		ics.WithNewLineWindows,
		ics.WithLineLength(75),
	)

	if err := os.WriteFile(outPath, []byte(out), 0644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}

	io.WriteString(os.Stderr, fmt.Sprintf("Done. events=%d, recurrencePropsRemoved=%d\n", len(events), recurrenceRemoved))
}
