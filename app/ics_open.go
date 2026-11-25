package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/emersion/go-ical"
	extcalendarbe "github.com/hkdb/aerion/extensions/calendar/backend"
	"github.com/hkdb/aerion/internal/logging"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Opening .ics files (Emailio addition). Emailio started with an .ics file,
// or handed one while running, shows the event in the calendar's new-event
// form so it can be added to a calendar.

// ICSCommandPrefix marks an .ics path passed from a second instance.
const ICSCommandPrefix = "ics:"

// maxICSFileSize is the largest .ics file Emailio will read.
const maxICSFileSize = 1 << 20 // 1 MB

// ICSEvent is the event read from an .ics file, for the event form.
type ICSEvent struct {
	FileName       string `json:"fileName"`
	Summary        string `json:"summary"`
	Location       string `json:"location"`
	Description    string `json:"description"`
	StartUnix      int64  `json:"startUnix"`
	EndUnix        int64  `json:"endUnix"`
	AllDay         bool   `json:"allDay"`
	OrganizerName  string `json:"organizerName"`
	OrganizerEmail string `json:"organizerEmail"`
	// Recurring is true when the event repeats; it is added once.
	Recurring bool `json:"recurring"`
	// EventCount is how many separate events the file holds; only the
	// first is shown.
	EventCount int `json:"eventCount"`
}

// IsICSPath reports whether a command-line argument looks like an .ics file.
func IsICSPath(arg string) bool {
	return strings.EqualFold(filepath.Ext(arg), ".ics")
}

// ReadICSFile reads and parses an .ics file.
func ReadICSFile(path string) (*ICSEvent, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("could not open the file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a file: %s", path)
	}
	if info.Size() > maxICSFileSize {
		return nil, fmt.Errorf("the file is too large (over 1 MB)")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read the file: %w", err)
	}
	ev, err := ParseICS(string(data))
	if err != nil {
		return nil, err
	}
	ev.FileName = filepath.Base(path)
	return ev, nil
}

// ParseICS reads the first event from .ics data.
func ParseICS(data string) (*ICSEvent, error) {
	parsed, err := extcalendarbe.ParseCalendarObject(data)
	if err != nil {
		return nil, fmt.Errorf("the file doesn't contain a calendar event Emailio can read: %w", err)
	}
	m := parsed.Master
	ev := &ICSEvent{
		Summary:     extcalendarbe.UnescapeText(m.Summary),
		Location:    extcalendarbe.UnescapeText(m.Location),
		Description: extcalendarbe.UnescapeText(m.Description),
		StartUnix:   m.DTStartUnix,
		EndUnix:     m.DTEndUnix,
		AllDay:      m.IsAllDay,
		Recurring:   m.RRuleText != "",
		EventCount:  countICSEvents(data),
	}
	if m.Organizer != nil {
		ev.OrganizerName = m.Organizer.CommonName
		ev.OrganizerEmail = m.Organizer.Email
	}
	return ev, nil
}

// countICSEvents counts the separate events in the data: one per UID, since
// exceptions to a repeating event share its UID.
func countICSEvents(data string) int {
	cal, err := ical.NewDecoder(strings.NewReader(data)).Decode()
	if err != nil {
		return 1
	}
	uids := map[string]bool{}
	for _, e := range cal.Events() {
		uid := ""
		if p := e.Props.Get(ical.PropUID); p != nil {
			uid = p.Value
		}
		uids[uid] = true
	}
	if len(uids) == 0 {
		return 1
	}
	return len(uids)
}

// GetPendingICS returns and clears the .ics file Emailio was started with.
// Returns nil when there is none.
func (a *App) GetPendingICS() (*ICSEvent, error) {
	path := a.PendingICSPath
	a.PendingICSPath = ""
	if path == "" {
		return nil, nil
	}
	return ReadICSFile(path)
}

// handleExternalICS handles an .ics file passed from a second instance.
func (a *App) handleExternalICS(path string) {
	log := logging.WithComponent("app")
	a.ShowWindow()

	ev, err := ReadICSFile(path)
	if err != nil {
		log.Warn().Err(err).Str("path", path).Msg("Failed to open .ics file")
		wailsRuntime.EventsEmit(a.ctx, "ics:error", map[string]interface{}{
			"fileName": filepath.Base(path),
			"error":    err.Error(),
		})
		return
	}
	wailsRuntime.EventsEmit(a.ctx, "ics:open", ev)
}
