package app

import (
	"os"
	"strings"

	"github.com/hkdb/aerion/internal/platform"
)

// Settings → General → Default email app (Emailio addition).

// DefaultMailAppStatus describes the current user's mail-app registration.
type DefaultMailAppStatus struct {
	// Supported is false on platforms where Settings can't register the app.
	Supported bool `json:"supported"`
	// Registered is true when a registration exists for the current user.
	Registered bool `json:"registered"`
	// RegisteredPath is the program the registration opens.
	RegisteredPath string `json:"registeredPath"`
	// CurrentPath is the program that is running now.
	CurrentPath string `json:"currentPath"`
	// MissingICS is true for a registration made before .ics files were
	// included; registering again adds them.
	MissingICS bool `json:"missingIcs"`
	// UpToDate is true when the registration opens the running program and
	// covers everything Emailio can open.
	UpToDate bool `json:"upToDate"`
}

// GetDefaultMailAppStatus reports whether Emailio is registered as a mail app.
func (a *App) GetDefaultMailAppStatus() DefaultMailAppStatus {
	status := DefaultMailAppStatus{Supported: platform.MailClientSupported}
	if !status.Supported {
		return status
	}
	status.RegisteredPath = platform.MailClientRegistration()
	status.Registered = status.RegisteredPath != ""
	if exe, err := os.Executable(); err == nil {
		status.CurrentPath = exe
	}
	status.MissingICS = status.Registered && !platform.MailClientHandlesICS()
	status.UpToDate = status.Registered && !status.MissingICS &&
		strings.EqualFold(status.RegisteredPath, status.CurrentPath)
	return status
}

// RegisterAsMailApp registers the running program as a mail app for the
// current user, then opens Windows Settings so the user can make it the default.
func (a *App) RegisterAsMailApp() error {
	if err := platform.RegisterMailClient(); err != nil {
		return err
	}
	return platform.OpenDefaultAppsSettings()
}

// UnregisterAsMailApp removes the current user's mail-app registration.
func (a *App) UnregisterAsMailApp() error {
	return platform.UnregisterMailClient()
}

// OpenDefaultAppsSettings opens Windows Settings → Default apps.
func (a *App) OpenDefaultAppsSettings() error {
	return platform.OpenDefaultAppsSettings()
}
