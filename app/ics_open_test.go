package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testInvite = "BEGIN:VCALENDAR\r\n" +
	"VERSION:2.0\r\n" +
	"PRODID:-//Test//EN\r\n" +
	"METHOD:REQUEST\r\n" +
	"BEGIN:VEVENT\r\n" +
	"UID:meeting-1@example.com\r\n" +
	"DTSTAMP:20261110T090000Z\r\n" +
	"DTSTART:20261120T140000Z\r\n" +
	"DTEND:20261120T150000Z\r\n" +
	"SUMMARY:Sample review\r\n" +
	"LOCATION:Istanbul office\r\n" +
	"DESCRIPTION:Bring the swatches\\, please.\r\n" +
	"ORGANIZER;CN=Zafer Selek:mailto:zafer@example.com\r\n" +
	"END:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

func TestParseICSInvite(t *testing.T) {
	ev, err := ParseICS(testInvite)
	if err != nil {
		t.Fatalf("ParseICS: %v", err)
	}
	if ev.Summary != "Sample review" || ev.Location != "Istanbul office" {
		t.Errorf("summary/location = %q / %q", ev.Summary, ev.Location)
	}
	if ev.Description != "Bring the swatches, please." {
		t.Errorf("description = %q", ev.Description)
	}
	want := time.Date(2026, 11, 20, 14, 0, 0, 0, time.UTC).Unix()
	if ev.StartUnix != want || ev.EndUnix != want+3600 {
		t.Errorf("start/end = %d/%d, want %d/%d", ev.StartUnix, ev.EndUnix, want, want+3600)
	}
	if ev.AllDay || ev.Recurring || ev.EventCount != 1 {
		t.Errorf("allDay=%v recurring=%v count=%d", ev.AllDay, ev.Recurring, ev.EventCount)
	}
	if ev.OrganizerName != "Zafer Selek" || ev.OrganizerEmail != "zafer@example.com" {
		t.Errorf("organizer = %q <%q>", ev.OrganizerName, ev.OrganizerEmail)
	}
}

func TestParseICSAllDayRecurringMultiple(t *testing.T) {
	data := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\n" +
		"BEGIN:VEVENT\r\nUID:a@x\r\nDTSTAMP:20261110T090000Z\r\nDTSTART;VALUE=DATE:20261201\r\nDTEND;VALUE=DATE:20261203\r\n" +
		"RRULE:FREQ=YEARLY\r\nSUMMARY:Trade fair\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:b@x\r\nDTSTAMP:20261110T090000Z\r\nDTSTART:20261205T100000Z\r\nSUMMARY:Other\r\nEND:VEVENT\r\n" +
		"END:VCALENDAR\r\n"
	ev, err := ParseICS(data)
	if err != nil {
		t.Fatalf("ParseICS: %v", err)
	}
	if !ev.AllDay || !ev.Recurring || ev.EventCount != 2 {
		t.Errorf("allDay=%v recurring=%v count=%d", ev.AllDay, ev.Recurring, ev.EventCount)
	}
	if ev.EndUnix-ev.StartUnix != 2*24*3600 {
		t.Errorf("all-day span = %d s, want two days", ev.EndUnix-ev.StartUnix)
	}
}

func TestReadICSFileRejectsBadInput(t *testing.T) {
	dir := t.TempDir()

	if _, err := ReadICSFile(filepath.Join(dir, "missing.ics")); err == nil {
		t.Error("missing file should fail")
	}

	notCal := filepath.Join(dir, "note.ics")
	_ = os.WriteFile(notCal, []byte("hello"), 0o600)
	if _, err := ReadICSFile(notCal); err == nil {
		t.Error("non-calendar content should fail")
	}

	big := filepath.Join(dir, "big.ics")
	_ = os.WriteFile(big, make([]byte, maxICSFileSize+1), 0o600)
	if _, err := ReadICSFile(big); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Errorf("oversized file: err = %v", err)
	}

	good := filepath.Join(dir, "Invite.ICS")
	_ = os.WriteFile(good, []byte(testInvite), 0o600)
	ev, err := ReadICSFile(good)
	if err != nil {
		t.Fatalf("ReadICSFile: %v", err)
	}
	if ev.FileName != "Invite.ICS" {
		t.Errorf("FileName = %q", ev.FileName)
	}
	if !IsICSPath(good) || IsICSPath("mailto:x@y") {
		t.Error("IsICSPath mismatch")
	}
}
