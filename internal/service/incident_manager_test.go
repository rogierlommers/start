package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"start/internal/config"
	"start/internal/repository"
)

func TestParseICalendar(t *testing.T) {
	location, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}

	calendar := "BEGIN:VCALENDAR\r\n" +
		"BEGIN:VEVENT\r\n" +
		"DTSTART;VALUE=DATE:20261009\r\n" +
		"DTEND;VALUE=DATE:20261010\r\n" +
		"SUMMARY:Alice Example\r\n" +
		"DESCRIPTION:Primary incident manager\\nCall +31 20 123\r\n" +
		"END:VEVENT\r\n" +
		"BEGIN:VEVENT\r\n" +
		"DTSTART;TZID=Europe/Amsterdam:20261009T180000\r\n" +
		"DTEND;TZID=Europe/Amsterdam:20261009T190000\r\n" +
		"SUMMARY:Hand-over to Bob \r\n" +
		" Example\r\n" +
		"LOCATION:Video call\r\n" +
		"END:VEVENT\r\n" +
		"END:VCALENDAR\r\n"

	events, err := parseICalendar(calendar, location)
	if err != nil {
		t.Fatalf("parseICalendar() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("parseICalendar() returned %d events, want 2", len(events))
	}
	if !events[0].AllDay || events[0].Summary != "Alice Example" || !strings.Contains(events[0].Description, "\nCall") {
		t.Fatalf("first event = %+v", events[0])
	}
	if events[1].Summary != "Hand-over to Bob Example" || events[1].Start.Hour() != 18 || events[1].Location != "Video call" {
		t.Fatalf("second event = %+v", events[1])
	}
}

func TestSendIncidentManagerSummaryForFollowingDay(t *testing.T) {
	calendar := "BEGIN:VCALENDAR\n" +
		"BEGIN:VEVENT\n" +
		"DTSTART;VALUE=DATE:20261002\n" +
		"DTEND;VALUE=DATE:20261003\n" +
		"RRULE:FREQ=WEEKLY;COUNT=3\n" +
		"SUMMARY:Alice Example\n" +
		"DESCRIPTION:Incident manager\n" +
		"END:VEVENT\n" +
		"BEGIN:VEVENT\n" +
		"DTSTART;VALUE=DATE:20261010\n" +
		"DTEND;VALUE=DATE:20261011\n" +
		"SUMMARY:Wrong day\n" +
		"END:VEVENT\n" +
		"END:VCALENDAR\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); !strings.Contains(got, "text/calendar") {
			t.Errorf("Accept header = %q", got)
		}
		_, _ = w.Write([]byte(calendar))
	}))
	defer server.Close()

	sender := &recordingSender{}
	svc := New(repository.NewMemoryStore(), sender, config.Config{
		MailerEmailWork:         "work@example.com",
		IncidentManagerICALURL:  server.URL,
		IncidentManagerNotifyAt: "17:00",
	})
	if err := svc.sendIncidentManagerSummary(context.Background(), time.Date(2026, 10, 8, 17, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("sendIncidentManagerSummary() error = %v", err)
	}

	if len(sender.messages) != 1 {
		t.Fatalf("sent %d messages, want 1", len(sender.messages))
	}
	msg := sender.messages[0]
	if msg.To != "work@example.com" || !strings.Contains(msg.Subject, "Friday, 9 October 2026") {
		t.Fatalf("message headers = To %q, Subject %q", msg.To, msg.Subject)
	}
	if !strings.Contains(msg.Body, "Alice Example") || strings.Contains(msg.Body, "Wrong day") {
		t.Fatalf("message body = %q", msg.Body)
	}
}

func TestNextDailyRunUsesLocalCalendarDay(t *testing.T) {
	location, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}

	// The following day starts daylight-saving time. The result should still be
	// 17:00 local time rather than 24 hours after the previous 17:00.
	now := time.Date(2026, 3, 28, 18, 0, 0, 0, location)
	next := nextDailyRun(now, 17, 0, location)
	_, offset := next.Zone()
	if next.Day() != 29 || next.Hour() != 17 || offset != 2*60*60 {
		t.Fatalf("nextDailyRun() = %s, want 2026-03-29 17:00 CEST", next)
	}
}

func TestRecurringEventHonorsExDate(t *testing.T) {
	location := time.UTC
	events, err := parseICalendar("BEGIN:VCALENDAR\n"+
		"BEGIN:VEVENT\n"+
		"DTSTART:20261002T090000Z\n"+
		"DTEND:20261002T170000Z\n"+
		"RRULE:FREQ=WEEKLY;COUNT=3\n"+
		"EXDATE:20261009T090000Z\n"+
		"SUMMARY:Alice Example\n"+
		"END:VEVENT\n"+
		"END:VCALENDAR\n", location)
	if err != nil {
		t.Fatalf("parseICalendar() error = %v", err)
	}

	start := time.Date(2026, 10, 9, 0, 0, 0, 0, location)
	occurrences, err := events[0].occurrencesBetween(start, start.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("occurrencesBetween() error = %v", err)
	}
	if len(occurrences) != 0 {
		t.Fatalf("occurrencesBetween() = %+v, want excluded occurrence", occurrences)
	}
}

func TestGetIncidentManagerOverviewReturnsFourteenLocalDays(t *testing.T) {
	calendar := "BEGIN:VCALENDAR\n" +
		"BEGIN:VEVENT\n" +
		"DTSTART;VALUE=DATE:20261008\n" +
		"DTEND;VALUE=DATE:20261009\n" +
		"RRULE:FREQ=DAILY;COUNT=3\n" +
		"SUMMARY:Alice Example\n" +
		"LOCATION:Primary rotation\n" +
		"END:VEVENT\n" +
		"END:VCALENDAR\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(calendar))
	}))
	defer server.Close()

	svc := New(repository.NewMemoryStore(), nil, config.Config{IncidentManagerICALURL: server.URL})
	overview, err := svc.GetIncidentManagerOverview(context.Background(), time.Date(2026, 10, 8, 22, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetIncidentManagerOverview() error = %v", err)
	}
	if overview.Timezone != "Europe/Amsterdam" || overview.RangeStart.Day() != 9 || overview.RangeEnd.Day() != 23 {
		t.Fatalf("overview range = %s to %s in %q", overview.RangeStart, overview.RangeEnd, overview.Timezone)
	}
	if len(overview.Duties) != 2 {
		t.Fatalf("overview duties = %+v, want two occurrences in range", overview.Duties)
	}
	if overview.Duties[0].Summary != "Alice Example" || overview.Duties[0].Location != "Primary rotation" || !overview.Duties[0].AllDay {
		t.Fatalf("first duty = %+v", overview.Duties[0])
	}
}

func TestGetIncidentManagerOverviewDisabled(t *testing.T) {
	svc := New(repository.NewMemoryStore(), nil, config.Config{})
	_, err := svc.GetIncidentManagerOverview(context.Background(), time.Now())
	if !errors.Is(err, ErrIncidentManagerDisabled) {
		t.Fatalf("GetIncidentManagerOverview() error = %v, want %v", err, ErrIncidentManagerDisabled)
	}
}
