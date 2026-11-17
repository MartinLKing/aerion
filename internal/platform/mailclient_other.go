//go:build !windows

package platform

import "errors"

// Registering as the system mail app from Settings is Windows-only (see
// mailclient_windows.go). Linux and macOS use their installers / desktop files.

const MailClientSupported = false

var errMailClientUnsupported = errors.New("platform: mail app registration is windows-only")

func RegisterMailClient() error      { return errMailClientUnsupported }
func UnregisterMailClient() error    { return errMailClientUnsupported }
func MailClientRegistration() string { return "" }
func MailClientHandlesICS() bool      { return false }
func OpenDefaultAppsSettings() error { return errMailClientUnsupported }
