package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"log/slog"
	"os"
	"regexp"
	"strings"

	ics "github.com/arran4/golang-ical"
)

var (
	reBR  = regexp.MustCompile(`(?i)<br\s*/?>`)
	reTag = regexp.MustCompile(`<[^>]+>`)
)

func normalizeUIDValue(propName string, value string) string {
	value = strings.TrimSpace(value)
	switch propName {
	case string(ics.ComponentPropertySummary), string(ics.ComponentPropertyDescription), string(ics.ComponentPropertyLocation):
		value = strings.ReplaceAll(value, "\r", "")
		if strings.Contains(value, "<") {
			return stripHTMLish(value)
		}
		return value
	}
	return strings.ReplaceAll(value, "\r", "")
}

func stableUID(e *ics.VEvent) string {
	startAt, _ := getPropValue(e, ics.ComponentPropertyDtStart)
	endAt, _ := getPropValue(e, ics.ComponentPropertyDtEnd)
	summary, _ := getPropValue(e, ics.ComponentPropertySummary)

	source := strings.Join([]string{
		normalizeUIDValue(string(ics.ComponentPropertyDtStart), startAt),
		normalizeUIDValue(string(ics.ComponentPropertyDtEnd), endAt),
		normalizeUIDValue(string(ics.ComponentPropertySummary), summary),
	}, "|")

	encoded := base64.RawURLEncoding.EncodeToString([]byte(source))
	return fmt.Sprintf("%s@calendar.lkj.io", encoded)
}

func dedupeUID(uid string, seen map[string]int) string {
	seen[uid]++
	if seen[uid] == 1 {
		return uid
	}

	localPart, domain, found := strings.Cut(uid, "@")
	if !found {
		return fmt.Sprintf("%s-%d", uid, seen[uid])
	}
	return fmt.Sprintf("%s-%d@%s", localPart, seen[uid], domain)
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
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if len(os.Args) != 3 {
		logger.Error("invalid arguments", "usage", "google-calendar-ics-fixer <input.ics> <output.ics>")
		os.Exit(2)
	}
	inPath := os.Args[1]
	outPath := os.Args[2]

	in, err := os.ReadFile(inPath)
	if err != nil {
		logger.Error("failed to read input", "path", inPath, "err", err)
		os.Exit(1)
	}

	cal, err := ics.ParseCalendar(bytes.NewReader(in))
	if err != nil {
		logger.Error("failed to parse calendar", "path", inPath, "err", err)
		os.Exit(1)
	}

	events := cal.Events()
	seenUIDs := make(map[string]int, len(events))
	recurrenceProps := []ics.ComponentProperty{
		ics.ComponentPropertyRrule,
		ics.ComponentPropertyRdate,
		ics.ComponentPropertyExrule,
		ics.ComponentPropertyExdate,
		ics.ComponentPropertyRecurrenceId,
	}

	recurrenceRemoved := 0
	for _, e := range events {
		// UID はイベント内容から一貫して生成する
		e.ReplaceProperty(ics.ComponentPropertyUniqueId, dedupeUID(stableUID(e), seenUIDs))

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
		logger.Error("failed to write output", "path", outPath, "err", err)
		os.Exit(1)
	}

	logger.Info("done", "events", len(events), "recurrencePropsRemoved", recurrenceRemoved, "output", outPath)
}
