package app

import (
	"context"
	"testing"
	"time"
)

func TestComposerCloseAsks(t *testing.T) {
	c := &ComposerApp{}
	// Still loading: nothing could answer, so it must be possible to close
	if c.BeforeClose(context.Background()) {
		t.Error("a composer that can't ask must close")
	}
	c.SetClosePromptReady(true)
	if !c.mustAskBeforeClose() {
		t.Error("an open composer must ask")
	}
	c.closeConfirmed.Store(true)
	if c.BeforeClose(context.Background()) {
		t.Error("an answered composer must close")
	}
}

func TestWaitOrTimeout(t *testing.T) {
	done := make(chan struct{})
	if waitOrTimeout(done, time.Millisecond) {
		t.Error("should time out")
	}
	close(done)
	if !waitOrTimeout(done, time.Second) {
		t.Error("should finish")
	}
}
