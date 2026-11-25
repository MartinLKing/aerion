package app

import (
	"testing"

	"github.com/hkdb/aerion/internal/message"
)

func TestParseAllICS(t *testing.T) {
	data := "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:x\n" +
		"BEGIN:VEVENT\nUID:out\nDTSTAMP:20260801T000000Z\nDTSTART:20260820T173000Z\nDTEND:20260820T193500Z\nSUMMARY:Flight out\nLOCATION:Cologne\nEND:VEVENT\n" +
		// No UID, as some senders do
		"BEGIN:VEVENT\nDTSTART:20260827T203500Z\nDTEND:20260827T224500Z\nSUMMARY:Flight back\nEND:VEVENT\n" +
		// A changed occurrence of the first
		"BEGIN:VEVENT\nUID:out\nRECURRENCE-ID:20260821T173000Z\nDTSTART:20260821T173000Z\nSUMMARY:Changed\nEND:VEVENT\n" +
		// Unreadable
		"BEGIN:VEVENT\nUID:bad\nSUMMARY:No start\nEND:VEVENT\n" +
		"END:VCALENDAR\n"
	got := ParseAllICS(data)
	if len(got) != 2 || got[0].Summary != "Flight out" || got[0].Location != "Cologne" || got[1].Summary != "Flight back" {
		t.Fatalf("got %+v", got)
	}
	if got[0].EndUnix-got[0].StartUnix != 125*60 {
		t.Errorf("duration = %d s", got[0].EndUnix-got[0].StartUnix)
	}
	if got := ParseAllICS("not a calendar"); len(got) != 0 {
		t.Errorf("got %+v", got)
	}
	if got := ParseAllICS(testInvite); len(got) != 1 || got[0].OrganizerEmail != "zafer@example.com" {
		t.Errorf("invite: %+v", got)
	}
}

func TestIsCalendarAttachment(t *testing.T) {
	for _, att := range []message.Attachment{
		{Filename: "x", ContentType: "text/calendar"}, {Filename: "x", ContentType: "text/x-vcalendar"},
		{Filename: "invite.ICS", ContentType: "application/octet-stream"},
	} {
		if !isCalendarAttachment(&att) {
			t.Errorf("%+v should count", att)
		}
	}
	if isCalendarAttachment(&message.Attachment{Filename: "a.pdf", ContentType: "application/pdf"}) {
		t.Error("a PDF is not a calendar file")
	}
}
