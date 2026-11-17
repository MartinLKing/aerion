//go:build windows

package platform

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/hkdb/aerion/internal/logging"
)

// Registering Emailio as a mail app for the current user (Emailio addition).
//
// The installer registers the app for all users (HKLM, needs admin). This
// writes the same entries under HKCU instead, pointing at the exe that is
// running, so a copy run without the installer shows up in Windows Settings →
// Default apps. Windows does not let an app make itself the default; the user
// picks it there.

const (
	mailClientName      = "Emailio"
	mailClientKey       = `Software\Clients\Mail\` + mailClientName
	mailClientCapsKey   = mailClientKey + `\Capabilities`
	mailtoProgID        = mailClientName + "Mailto"
	mailtoProgIDKey     = `Software\Classes\` + mailtoProgID
	registeredAppsKey   = `Software\RegisteredApplications`
	mailClientCommandSz = `"%s" "%%1"`
)

// MailClientSupported reports whether this OS supports RegisterMailClient.
const MailClientSupported = true

// RegisterMailClient writes the mail-app registration for the current user.
func RegisterMailClient() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to find the Emailio program file: %w", err)
	}
	command := fmt.Sprintf(mailClientCommandSz, exe)
	icon := exe + ",0"

	values := []struct {
		path, name, value string
	}{
		{mailClientKey, "", mailClientName},
		{mailClientKey + `\shell\open\command`, "", command},
		{mailClientCapsKey, "ApplicationName", mailClientName},
		{mailClientCapsKey, "ApplicationDescription", "Email client"},
		{mailClientCapsKey, "ApplicationIcon", icon},
		{mailClientCapsKey + `\URLAssociations`, "mailto", mailtoProgID},
		{mailtoProgIDKey, "", "URL:MailTo Protocol"},
		{mailtoProgIDKey, "URL Protocol", ""},
		{mailtoProgIDKey + `\DefaultIcon`, "", icon},
		{mailtoProgIDKey + `\shell\open\command`, "", command},
		{registeredAppsKey, mailClientName, mailClientCapsKey},
	}
	for _, v := range values {
		if err := setUserString(v.path, v.name, v.value); err != nil {
			return err
		}
	}

	notifyAssociationsChanged()
	log := logging.WithComponent("mailclient")
	log.Info().Str("exe", exe).Msg("Registered as mail app for the current user")
	return nil
}

// UnregisterMailClient removes what RegisterMailClient wrote.
func UnregisterMailClient() error {
	if key, err := registry.OpenKey(registry.CURRENT_USER, registeredAppsKey, registry.SET_VALUE); err == nil {
		_ = key.DeleteValue(mailClientName)
		key.Close()
	}
	for _, path := range []string{mailClientKey, mailtoProgIDKey} {
		if err := deleteUserKeyTree(path); err != nil {
			return err
		}
	}
	notifyAssociationsChanged()
	log := logging.WithComponent("mailclient")
	log.Info().Msg("Removed mail app registration for the current user")
	return nil
}

// MailClientRegistration returns the program path the current user's
// registration points at, or "" when Emailio is not registered.
func MailClientRegistration() string {
	key, err := registry.OpenKey(registry.CURRENT_USER, mailtoProgIDKey+`\shell\open\command`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()
	command, _, err := key.GetStringValue("")
	if err != nil {
		return ""
	}
	// `"C:\path\emailio.exe" "%1"` -> C:\path\emailio.exe
	if strings.HasPrefix(command, `"`) {
		if end := strings.Index(command[1:], `"`); end >= 0 {
			return command[1 : end+1]
		}
	}
	return command
}

// OpenDefaultAppsSettings opens Windows Settings at Emailio's Default apps
// page, falling back to the Default apps list.
func OpenDefaultAppsSettings() error {
	if err := OpenURLWindows("ms-settings:defaultapps?registeredAppUser=" + mailClientName); err == nil {
		return nil
	}
	return OpenURLWindows("ms-settings:defaultapps")
}

func setUserString(path, name, value string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to create registry key %s: %w", path, err)
	}
	defer key.Close()
	if err := key.SetStringValue(name, value); err != nil {
		return fmt.Errorf("failed to set registry value %s\\%s: %w", path, name, err)
	}
	return nil
}

// deleteUserKeyTree deletes a key under HKCU and everything below it.
// registry.DeleteKey only removes keys without subkeys.
func deleteUserKeyTree(path string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil // already gone
	}
	subkeys, _ := key.ReadSubKeyNames(-1)
	key.Close()
	for _, sub := range subkeys {
		if err := deleteUserKeyTree(path + `\` + sub); err != nil {
			return err
		}
	}
	if err := registry.DeleteKey(registry.CURRENT_USER, path); err != nil {
		return fmt.Errorf("failed to delete registry key %s: %w", path, err)
	}
	return nil
}

// notifyAssociationsChanged tells Explorer and Settings to re-read file and
// protocol associations (SHChangeNotify with SHCNE_ASSOCCHANGED).
func notifyAssociationsChanged() {
	const shcneAssocChanged = 0x08000000
	proc := windows.NewLazySystemDLL("shell32.dll").NewProc("SHChangeNotify")
	if proc.Find() != nil {
		return
	}
	_, _, _ = proc.Call(shcneAssocChanged, 0, 0, 0)
}
