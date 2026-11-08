package app

import (
	"errors"
	"testing"
)

func TestChildFolderPath(t *testing.T) {
	tests := []struct {
		parent, delim, name string
		want                string
		wantErr             bool
	}{
		{"", "/", "Receipts", "Receipts", false},
		{"", "/", "  Receipts  ", "Receipts", false},
		{"Work", "/", "Invoices", "Work/Invoices", false},
		{"Work/2026", ".", "Q1", "Work/2026.Q1", false},
		{"", "/", "", "", true},
		{"", "/", "   ", "", true},
		{"", "/", "a/b", "", true},
		{"Work", ".", "a.b", "", true},
		{"", "/", "inbox", "", true},
		{"Work", "/", "Inbox", "Work/Inbox", false},
		{"", "", "Flat", "Flat", false},
	}
	for _, tt := range tests {
		got, err := childFolderPath(tt.parent, tt.delim, tt.name)
		if (err != nil) != tt.wantErr {
			t.Errorf("childFolderPath(%q, %q, %q) error = %v, wantErr %v", tt.parent, tt.delim, tt.name, err, tt.wantErr)
			continue
		}
		if err != nil && !errors.Is(err, errInvalidFolderName) {
			t.Errorf("error %v should wrap errInvalidFolderName", err)
		}
		if got != tt.want {
			t.Errorf("childFolderPath(%q, %q, %q) = %q, want %q", tt.parent, tt.delim, tt.name, got, tt.want)
		}
	}
}

func TestParentFolderPath(t *testing.T) {
	tests := []struct{ path, delim, want string }{
		{"Receipts", "/", ""},
		{"Work/Invoices", "/", "Work"},
		{"Work/2026/Q1", "/", "Work/2026"},
		{"Work.Q1", ".", "Work"},
		{"Flat", "", ""},
	}
	for _, tt := range tests {
		if got := parentFolderPath(tt.path, tt.delim); got != tt.want {
			t.Errorf("parentFolderPath(%q, %q) = %q, want %q", tt.path, tt.delim, got, tt.want)
		}
	}
}

func TestRenamedFolderPath(t *testing.T) {
	tests := []struct {
		path, oldPath, newPath, delim string
		want                          string
		changed                       bool
	}{
		{"Work", "Work", "Job", "/", "Job", true},
		{"Work/Invoices", "Work", "Job", "/", "Job/Invoices", true},
		{"Work/2026/Q1", "Work", "Job", "/", "Job/2026/Q1", true},
		{"Workshop", "Work", "Job", "/", "Workshop", false},
		{"Other/Work", "Work", "Job", "/", "Other/Work", false},
		{"Work.Q1", "Work", "Job", ".", "Job.Q1", true},
	}
	for _, tt := range tests {
		got, changed := renamedFolderPath(tt.path, tt.oldPath, tt.newPath, tt.delim)
		if got != tt.want || changed != tt.changed {
			t.Errorf("renamedFolderPath(%q, %q, %q) = %q, %v; want %q, %v", tt.path, tt.oldPath, tt.newPath, got, changed, tt.want, tt.changed)
		}
	}
}

func TestFriendlyFolderError(t *testing.T) {
	notEmpty := &folderNotEmptyError{count: 3}
	if got := friendlyFolderError(notEmpty); got != error(notEmpty) {
		t.Errorf("not-empty error should pass through unchanged, got %v", got)
	}
	if got := friendlyFolderError(errors.New("NO [ALREADYEXISTS]")); got.Error() != "the mail server refused: NO [ALREADYEXISTS]" {
		t.Errorf("server error = %q", got.Error())
	}
}
