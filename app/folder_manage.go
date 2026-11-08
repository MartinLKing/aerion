package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hkdb/aerion/internal/folder"
	"github.com/hkdb/aerion/internal/imap"
	"github.com/hkdb/aerion/internal/logging"
	"github.com/rs/zerolog"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Creating, renaming and deleting folders. Only the user's own folders can be
// changed: Inbox, Sent, Drafts, Trash, Spam, Archive and the like are left alone.

const folderManageTimeout = 60 * time.Second

func folderLogger() *zerolog.Logger {
	log := logging.WithComponent("app.folders")
	return &log
}

// ============================================================================
// Folder management API - Exposed to frontend via Wails bindings
// ============================================================================

// CreateFolder creates a folder on the server, at the top level or inside one of
// the user's folders, and returns it once the folder list has been refreshed.
func (a *App) CreateFolder(accountID, parentFolderID, name string) (*folder.Folder, error) {
	parentPath := ""
	if parentFolderID != "" {
		parent, err := a.userFolder(accountID, parentFolderID)
		if err != nil {
			return nil, err
		}
		parentPath = parent.Path
	}

	var path string
	err := a.withIMAPRetry(accountID, func(conn *imap.Client) error {
		delim, err := conn.HierarchyDelimiter()
		if err != nil {
			return err
		}
		if path, err = childFolderPath(parentPath, delim, name); err != nil {
			return err
		}
		if err := conn.CreateMailbox(path); err != nil {
			return err
		}
		// Subscribed, so it shows up in other mail apps too
		if err := conn.Subscribe(path); err != nil {
			folderLogger().Warn().Err(err).Str("path", path).Msg("Failed to subscribe to new folder")
		}
		return nil
	})
	if err != nil {
		return nil, friendlyFolderError(err)
	}

	a.refreshFolderList(accountID)
	created, err := a.folderStore.GetByPath(accountID, path)
	if err != nil {
		return nil, err
	}
	return created, nil
}

// RenameFolder renames one of the user's folders. Its subfolders move with it,
// and the local records are updated in place, so filter rules pointing at these
// folders keep working.
func (a *App) RenameFolder(accountID, folderID, newName string) error {
	f, err := a.userFolder(accountID, folderID)
	if err != nil {
		return err
	}

	var oldPath, newPath, delim string
	err = a.withIMAPRetry(accountID, func(conn *imap.Client) error {
		var err error
		if delim, err = conn.HierarchyDelimiter(); err != nil {
			return err
		}
		oldPath = f.Path
		if newPath, err = childFolderPath(parentFolderPath(oldPath, delim), delim, newName); err != nil {
			return err
		}
		if newPath == oldPath {
			return nil
		}
		return conn.RenameMailbox(oldPath, newPath)
	})
	if err != nil {
		return friendlyFolderError(err)
	}
	if newPath == oldPath {
		return nil
	}

	// Move the local records to their new paths before the refresh, which
	// matches folders by path and would otherwise replace them with new ones
	folders, err := a.folderStore.List(accountID)
	if err != nil {
		return err
	}
	for _, local := range folders {
		updated, changed := renamedFolderPath(local.Path, oldPath, newPath, delim)
		if !changed {
			continue
		}
		local.Path = updated
		if local.ID == f.ID {
			local.Name = strings.TrimSpace(newName)
		}
		if err := a.folderStore.Update(local); err != nil {
			folderLogger().Warn().Err(err).Str("path", updated).Msg("Failed to update renamed folder locally")
		}
	}

	a.refreshFolderList(accountID)
	return nil
}

// DeleteFolder deletes one of the user's folders, but only when it's empty, has
// no subfolders and no filter rule files mail into it. Some servers delete the
// messages in a folder along with it, so nothing is deleted that holds mail.
func (a *App) DeleteFolder(accountID, folderID string) error {
	f, err := a.userFolder(accountID, folderID)
	if err != nil {
		return err
	}

	folders, err := a.folderStore.List(accountID)
	if err != nil {
		return err
	}
	for _, other := range folders {
		if other.ParentID == f.ID {
			return errors.New("this folder has subfolders: delete or move them first")
		}
	}

	if rules, err := a.settingsStore.GetFilterRules(); err == nil {
		for _, rule := range rules {
			if rule.MoveToFolderID == f.ID {
				return errors.New("a filter rule files mail into this folder: change or delete the rule first")
			}
		}
	}

	err = a.withIMAPRetry(accountID, func(conn *imap.Client) error {
		ctx, cancel := context.WithTimeout(a.ctx, folderManageTimeout)
		defer cancel()
		// Ask the server, not the local copy, which may not hold everything
		status, err := conn.GetMailboxStatus(ctx, f.Path)
		if err != nil {
			return err
		}
		if status.Messages > 0 {
			return &folderNotEmptyError{count: status.Messages}
		}
		if err := conn.Unsubscribe(f.Path); err != nil {
			folderLogger().Debug().Err(err).Str("path", f.Path).Msg("Unsubscribe before delete failed")
		}
		return conn.DeleteMailbox(f.Path)
	})
	if err != nil {
		return friendlyFolderError(err)
	}

	if err := a.folderStore.Delete(f.ID); err != nil {
		folderLogger().Warn().Err(err).Str("path", f.Path).Msg("Failed to remove deleted folder locally")
	}
	a.refreshFolderList(accountID)
	return nil
}

// userFolder loads a folder and checks that the user may change it.
func (a *App) userFolder(accountID, folderID string) (*folder.Folder, error) {
	f, err := a.folderStore.Get(folderID)
	if err != nil {
		return nil, fmt.Errorf("failed to load folder: %w", err)
	}
	if f == nil || f.AccountID != accountID {
		return nil, errors.New("folder not found")
	}
	if f.Type != folder.TypeFolder {
		return nil, errors.New("system folders such as Inbox, Sent and Trash can't be changed")
	}
	return f, nil
}

// refreshFolderList re-reads the folder list from the server and tells the
// sidebar. Failures are only logged: the change itself has already happened.
func (a *App) refreshFolderList(accountID string) {
	ctx, cancel := context.WithTimeout(a.ctx, folderManageTimeout)
	defer cancel()
	if err := a.syncEngine.SyncFolders(ctx, accountID); err != nil {
		folderLogger().Warn().Err(err).Str("account", accountID).Msg("Failed to refresh folder list")
	}
	wailsRuntime.EventsEmit(a.ctx, "folders:changed", map[string]interface{}{"accountId": accountID})
}

type folderNotEmptyError struct{ count uint32 }

func (e *folderNotEmptyError) Error() string {
	return fmt.Sprintf("this folder still holds %d message(s): move or delete them first", e.count)
}

// friendlyFolderError keeps the app's own messages as they are and gives server
// errors a readable prefix.
func friendlyFolderError(err error) error {
	var notEmpty *folderNotEmptyError
	if errors.As(err, &notEmpty) || errors.Is(err, errInvalidFolderName) {
		return err
	}
	return fmt.Errorf("the mail server refused: %w", err)
}

var errInvalidFolderName = errors.New("invalid folder name")

// childFolderPath builds the full path for a folder called name inside
// parentPath ("" for the top level), after checking the name is usable.
func childFolderPath(parentPath, delim, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("%w: enter a name", errInvalidFolderName)
	}
	if delim != "" && strings.Contains(name, delim) {
		return "", fmt.Errorf("%w: a folder name can't contain %q", errInvalidFolderName, delim)
	}
	if parentPath == "" && strings.EqualFold(name, "INBOX") {
		return "", fmt.Errorf("%w: that name is reserved", errInvalidFolderName)
	}
	if parentPath == "" {
		return name, nil
	}
	return parentPath + delim + name, nil
}

// parentFolderPath returns the path above path, or "" at the top level.
func parentFolderPath(path, delim string) string {
	if delim == "" {
		return ""
	}
	i := strings.LastIndex(path, delim)
	if i < 0 {
		return ""
	}
	return path[:i]
}

// renamedFolderPath moves path to its new place when it is the renamed folder
// or one of its subfolders.
func renamedFolderPath(path, oldPath, newPath, delim string) (string, bool) {
	if path == oldPath {
		return newPath, true
	}
	if delim != "" && strings.HasPrefix(path, oldPath+delim) {
		return newPath + path[len(oldPath):], true
	}
	return path, false
}
