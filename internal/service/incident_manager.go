package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	// Bundle zoneinfo so the fixed business timezone works in minimal containers.
	_ "time/tzdata"

	"start/internal/mailer"

	"github.com/sirupsen/logrus"
	rrule "github.com/teambition/rrule-go"
)

const (
	maxCalendarFeedBytes        = 2 << 20
	incidentManagerTimezone     = "Europe/Amsterdam"
	incidentManagerOverviewDays = 14
)

var ErrIncidentManagerDisabled = errors.New("incident-manager calendar is not configured")

type IncidentManagerDuty struct {
	Summary     string
	Description string
	Location    string
	Start       time.Time
	End         time.Time
	AllDay      bool
}

type IncidentManagerOverview struct {
	Timezone   string
	RangeStart time.Time
	RangeEnd   time.Time
	Duties     []IncidentManagerDuty
}

type incidentManagerEvent struct {
	Summary        string
	Description    string
	Location       string
	Start          time.Time
	End            time.Time
	AllDay         bool
	Cancelled      bool
	RecurrenceRule string
	ExDates        []time.Time
}

// StartIncidentManagerWorker sends a summary of the following day's duty calendar
// at the configured local wall-clock time. An empty feed URL disables the worker.
func (s *Service) StartIncidentManagerWorker() {
	if strings.TrimSpace(s.cfg.IncidentManagerICALURL) == "" {
		logrus.Info("incident-manager notifications disabled")
		return
	}

	location, err := time.LoadLocation(incidentManagerTimezone)
	if err != nil {
		logrus.Errorf("incident-manager notifications disabled: invalid timezone: %v", err)
		return
	}
	scheduledTime, err := time.Parse("15:04", s.cfg.IncidentManagerNotifyAt)
	if err != nil {
		logrus.Errorf("incident-manager notifications disabled: invalid notification time: %v", err)
		return
	}

	go func() {
		for {
			now := time.Now().In(location)
			next := nextDailyRun(now, scheduledTime.Hour(), scheduledTime.Minute(), location)
			timer := time.NewTimer(time.Until(next))

			select {
			case <-timer.C:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				err := s.sendIncidentManagerSummary(ctx, next)
				cancel()
				if err != nil {
					logrus.Errorf("incident-manager notification failed: %v", err)
				} else {
					logrus.Info("incident-manager notification sent")
				}
			case <-s.done:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return
			}
		}
	}()
}

func nextDailyRun(now time.Time, hour, minute int, location *time.Location) time.Time {
	now = now.In(location)
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, location)
	if !next.After(now) {
		next = time.Date(now.Year(), now.Month(), now.Day()+1, hour, minute, 0, 0, location)
	}
	return next
}

func (s *Service) sendIncidentManagerSummary(ctx context.Context, now time.Time) error {
	if _, ok := s.mailer.(mailer.DisabledSender); ok {
		return ErrDisabledMailer
	}

	location, err := time.LoadLocation(incidentManagerTimezone)
	if err != nil {
		return fmt.Errorf("load notification timezone: %w", err)
	}
	tomorrow := now.In(location).AddDate(0, 0, 1)
	dayStart := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, location)
	dayEnd := dayStart.AddDate(0, 0, 1)

	matching, err := s.incidentManagerEventsBetween(ctx, dayStart, dayEnd, location)
	if err != nil {
		return err
	}

	msg := mailer.Message{
		To:      s.cfg.MailerEmailWork,
		Subject: fmt.Sprintf("Incident manager for %s", dayStart.Format("Monday, 2 January 2006")),
		Body:    formatIncidentManagerSummary(dayStart, matching, location),
	}
	if err := s.mailer.Send(ctx, msg); err != nil {
		return fmt.Errorf("send incident-manager email: %w", err)
	}
	return nil
}

// GetIncidentManagerOverview returns duties overlapping today and the next 13
// calendar days in the incident-manager business timezone.
func (s *Service) GetIncidentManagerOverview(ctx context.Context, now time.Time) (IncidentManagerOverview, error) {
	if strings.TrimSpace(s.cfg.IncidentManagerICALURL) == "" {
		return IncidentManagerOverview{}, ErrIncidentManagerDisabled
	}

	location, err := time.LoadLocation(incidentManagerTimezone)
	if err != nil {
		return IncidentManagerOverview{}, fmt.Errorf("load incident-manager timezone: %w", err)
	}
	localNow := now.In(location)
	rangeStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	rangeEnd := rangeStart.AddDate(0, 0, incidentManagerOverviewDays)

	events, err := s.incidentManagerEventsBetween(ctx, rangeStart, rangeEnd, location)
	if err != nil {
		return IncidentManagerOverview{}, err
	}

	duties := make([]IncidentManagerDuty, len(events))
	for i, event := range events {
		duties[i] = IncidentManagerDuty{
			Summary:     event.Summary,
			Description: event.Description,
			Location:    event.Location,
			Start:       event.Start,
			End:         event.End,
			AllDay:      event.AllDay,
		}
	}

	return IncidentManagerOverview{
		Timezone:   incidentManagerTimezone,
		RangeStart: rangeStart,
		RangeEnd:   rangeEnd,
		Duties:     duties,
	}, nil
}

func (s *Service) incidentManagerEventsBetween(ctx context.Context, start, end time.Time, location *time.Location) ([]incidentManagerEvent, error) {
	events, err := s.fetchIncidentManagerEvents(ctx, location)
	if err != nil {
		return nil, err
	}

	var matching []incidentManagerEvent
	for _, event := range events {
		occurrences, err := event.occurrencesBetween(start, end)
		if err != nil {
			return nil, fmt.Errorf("expand calendar recurrence: %w", err)
		}
		for _, occurrence := range occurrences {
			if !occurrence.Cancelled && occurrence.Start.Before(end) && occurrence.End.After(start) {
				matching = append(matching, occurrence)
			}
		}
	}
	sort.Slice(matching, func(i, j int) bool { return matching[i].Start.Before(matching[j].Start) })
	return matching, nil
}

func (s *Service) fetchIncidentManagerEvents(ctx context.Context, defaultLocation *time.Location) ([]incidentManagerEvent, error) {
	feedURL, err := url.Parse(s.cfg.IncidentManagerICALURL)
	if err != nil || (feedURL.Scheme != "http" && feedURL.Scheme != "https") || feedURL.Host == "" {
		return nil, errors.New("incident-manager calendar URL must be an absolute HTTP or HTTPS URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create calendar request: %w", err)
	}
	req.Header.Set("Accept", "text/calendar, application/ics;q=0.9, text/plain;q=0.5")

	client := s.httpClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download incident-manager calendar: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download incident-manager calendar: unexpected HTTP status %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxCalendarFeedBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read incident-manager calendar: %w", err)
	}
	if len(data) > maxCalendarFeedBytes {
		return nil, fmt.Errorf("incident-manager calendar exceeds %d bytes", maxCalendarFeedBytes)
	}

	events, err := parseICalendar(string(data), defaultLocation)
	if err != nil {
		return nil, fmt.Errorf("parse incident-manager calendar: %w", err)
	}
	return events, nil
}

func parseICalendar(data string, defaultLocation *time.Location) ([]incidentManagerEvent, error) {
	lines := unfoldICalendarLines(data)
	var events []incidentManagerEvent
	var current *incidentManagerEvent
	var startSet, endSet bool
	seenCalendar := false

	for _, line := range lines {
		nameAndParams, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		parts := strings.Split(nameAndParams, ";")
		name := strings.ToUpper(parts[0])
		params := make(map[string]string)
		for _, raw := range parts[1:] {
			key, paramValue, ok := strings.Cut(raw, "=")
			if ok {
				params[strings.ToUpper(key)] = strings.Trim(paramValue, `"`)
			}
		}

		switch {
		case name == "BEGIN" && strings.EqualFold(value, "VCALENDAR"):
			seenCalendar = true
		case name == "BEGIN" && strings.EqualFold(value, "VEVENT"):
			current = &incidentManagerEvent{}
			startSet, endSet = false, false
		case name == "END" && strings.EqualFold(value, "VEVENT"):
			if current == nil {
				continue
			}
			if !startSet {
				return nil, errors.New("VEVENT is missing DTSTART")
			}
			if !endSet {
				if current.AllDay {
					current.End = current.Start.AddDate(0, 0, 1)
				} else {
					current.End = current.Start.Add(time.Nanosecond)
				}
			}
			if !current.End.After(current.Start) {
				return nil, errors.New("VEVENT has DTEND before DTSTART")
			}
			events = append(events, *current)
			current = nil
		case current != nil && name == "SUMMARY":
			current.Summary = unescapeICalendarText(value)
		case current != nil && name == "DESCRIPTION":
			current.Description = unescapeICalendarText(value)
		case current != nil && name == "LOCATION":
			current.Location = unescapeICalendarText(value)
		case current != nil && name == "STATUS":
			current.Cancelled = strings.EqualFold(strings.TrimSpace(value), "CANCELLED")
		case current != nil && name == "RRULE":
			current.RecurrenceRule = strings.TrimSpace(value)
		case current != nil && name == "EXDATE":
			for _, rawDate := range strings.Split(value, ",") {
				parsed, _, err := parseICalendarTime(rawDate, params, defaultLocation)
				if err != nil {
					return nil, fmt.Errorf("EXDATE: %w", err)
				}
				current.ExDates = append(current.ExDates, parsed)
			}
		case current != nil && (name == "DTSTART" || name == "DTEND"):
			parsed, allDay, err := parseICalendarTime(value, params, defaultLocation)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			if name == "DTSTART" {
				current.Start, current.AllDay, startSet = parsed, allDay, true
			} else {
				current.End, endSet = parsed, true
			}
		}
	}
	if !seenCalendar {
		return nil, errors.New("feed is not an iCalendar VCALENDAR")
	}
	if current != nil {
		return nil, errors.New("unterminated VEVENT")
	}
	return events, nil
}

func (event incidentManagerEvent) occurrencesBetween(start, end time.Time) ([]incidentManagerEvent, error) {
	if event.RecurrenceRule == "" {
		return []incidentManagerEvent{event}, nil
	}

	option, err := rrule.StrToROptionInLocation(event.RecurrenceRule, event.Start.Location())
	if err != nil {
		return nil, fmt.Errorf("invalid RRULE %q: %w", event.RecurrenceRule, err)
	}
	rule, err := rrule.NewRRule(*option)
	if err != nil {
		return nil, fmt.Errorf("invalid RRULE %q: %w", event.RecurrenceRule, err)
	}
	rule.DTStart(event.Start)

	duration := event.End.Sub(event.Start)
	starts := rule.Between(start.Add(-duration), end, true)
	occurrences := make([]incidentManagerEvent, 0, len(starts))
	for _, occurrenceStart := range starts {
		if containsTime(event.ExDates, occurrenceStart) {
			continue
		}
		occurrence := event
		occurrence.Start = occurrenceStart
		occurrence.End = occurrenceStart.Add(duration)
		occurrence.RecurrenceRule = ""
		occurrence.ExDates = nil
		occurrences = append(occurrences, occurrence)
	}
	return occurrences, nil
}

func containsTime(times []time.Time, target time.Time) bool {
	for _, candidate := range times {
		if candidate.Equal(target) {
			return true
		}
	}
	return false
}

func unfoldICalendarLines(data string) []string {
	rawLines := strings.Split(strings.ReplaceAll(data, "\r\n", "\n"), "\n")
	var lines []string
	for _, rawLine := range rawLines {
		line := strings.TrimSuffix(rawLine, "\r")
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') && len(lines) > 0 {
			lines[len(lines)-1] += line[1:]
		} else {
			lines = append(lines, line)
		}
	}
	return lines
}

func parseICalendarTime(value string, params map[string]string, defaultLocation *time.Location) (time.Time, bool, error) {
	value = strings.TrimSpace(value)
	if strings.EqualFold(params["VALUE"], "DATE") || len(value) == len("20060102") {
		parsed, err := time.ParseInLocation("20060102", value, defaultLocation)
		return parsed, true, err
	}
	if strings.HasSuffix(value, "Z") {
		parsed, err := time.Parse("20060102T150405Z", value)
		return parsed, false, err
	}

	location := defaultLocation
	if tzid := params["TZID"]; tzid != "" {
		var err error
		location, err = time.LoadLocation(tzid)
		if err != nil {
			return time.Time{}, false, fmt.Errorf("load TZID %q: %w", tzid, err)
		}
	}
	parsed, err := time.ParseInLocation("20060102T150405", value, location)
	return parsed, false, err
}

func unescapeICalendarText(value string) string {
	replacer := strings.NewReplacer(`\n`, "\n", `\N`, "\n", `\,`, ",", `\;`, ";", `\\`, `\`)
	return strings.TrimSpace(replacer.Replace(value))
}

func formatIncidentManagerSummary(day time.Time, events []incidentManagerEvent, location *time.Location) string {
	var body strings.Builder
	fmt.Fprintf(&body, "Incident manager duty for %s:\n", day.Format("Monday, 2 January 2006"))
	if len(events) == 0 {
		body.WriteString("\nNo incident manager event was found in the calendar.\n")
		return body.String()
	}

	for _, event := range events {
		summary := strings.TrimSpace(event.Summary)
		if summary == "" {
			summary = "Untitled duty event"
		}
		fmt.Fprintf(&body, "\n- %s\n", summary)
		if event.AllDay {
			body.WriteString("  Time: all day\n")
		} else {
			fmt.Fprintf(&body, "  Time: %s–%s %s\n", event.Start.In(location).Format("15:04"), event.End.In(location).Format("15:04"), location.String())
		}
		if event.Location != "" {
			fmt.Fprintf(&body, "  Location: %s\n", event.Location)
		}
		if event.Description != "" {
			fmt.Fprintf(&body, "  Details: %s\n", strings.ReplaceAll(event.Description, "\n", "\n  "))
		}
	}
	return body.String()
}
