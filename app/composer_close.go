package app

import (
	"context"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Closing the pop-out composer (Emailio addition). However the window is closed
// (title bar, Alt+F4, taskbar), the composer asks what to do with the draft.

// draftUploadWait bounds how long a closing composer stays in the background to
// finish uploading its draft. If it runs out, the draft is kept locally and the
// main window uploads it when Emailio next starts.
const draftUploadWait = 20 * time.Second

// mustAskBeforeClose reports whether a close must be stopped to ask first.
func (c *ComposerApp) mustAskBeforeClose() bool {
	return c.closePromptReady.Load() && !c.closeConfirmed.Load()
}

// BeforeClose is called by Wails for every attempt to close the window, and by
// Quit. Returning true keeps the window open.
func (c *ComposerApp) BeforeClose(ctx context.Context) bool {
	if !c.mustAskBeforeClose() {
		return false
	}
	// Closed from the taskbar it may be minimized, hiding the question. Not on
	// this thread: it's the window's own, which the calls below need.
	go func() {
		// Only then: restoring would also shrink a maximized window
		if wailsRuntime.WindowIsMinimised(ctx) {
			wailsRuntime.WindowUnminimise(ctx)
		}
		wailsRuntime.WindowShow(ctx)
		wailsRuntime.EventsEmit(ctx, "composer:close-requested")
	}()
	return true
}

// SetClosePromptReady tells whether the composer is able to ask. While it is
// still loading, or failed to load, the window closes without asking.
func (c *ComposerApp) SetClosePromptReady(ready bool) {
	c.closePromptReady.Store(ready)
}

// CloseWindow closes the window without asking; the composer has asked already
// or has nothing to ask. A draft still uploading is finished first, out of sight.
func (c *ComposerApp) CloseWindow() {
	if c.closeConfirmed.Swap(true) {
		return
	}
	c.draftSyncMu.Lock()
	uploading := c.draftSyncDone
	c.draftSyncMu.Unlock()
	if uploading == nil {
		wailsRuntime.Quit(c.ctx)
		return
	}
	wailsRuntime.WindowHide(c.ctx)
	go func() {
		defer recoverPanic("composer", "finish draft upload before closing")
		waitOrTimeout(uploading, draftUploadWait)
		wailsRuntime.Quit(c.ctx)
	}()
}

// waitOrTimeout waits for done, at most for limit.
func waitOrTimeout(done <-chan struct{}, limit time.Duration) bool {
	select {
	case <-done:
		return true
	case <-time.After(limit):
		return false
	}
}
