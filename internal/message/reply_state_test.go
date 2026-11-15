package message

import (
	"testing"
	"time"
)

func seedReplyStateMsg(t *testing.T, s *Store, id, folderID, messageID, inReplyTo, subject string, uid uint32, date time.Time) {
	t.Helper()
	m := &Message{
		ID: id, AccountID: "acct-1", FolderID: folderID, UID: uid,
		MessageID: messageID, InReplyTo: inReplyTo, ThreadID: messageID,
		Subject: subject, Date: date,
	}
	if err := s.Create(m); err != nil {
		t.Fatalf("Create %s: %v", id, err)
	}
}

func replyStateByThread(t *testing.T, s *Store) map[string]*Conversation {
	t.Helper()
	convs, err := s.ListConversationsByFolder("inbox-1", 0, 50, "newest", "")
	if err != nil {
		t.Fatalf("ListConversationsByFolder: %v", err)
	}
	out := make(map[string]*Conversation)
	for _, c := range convs {
		out[c.ThreadID] = c
	}
	return out
}

func TestConversationReplyState(t *testing.T) {
	s, db := newExpandCopiesTestStore(t)
	now := time.Now().UTC()

	// Untouched message.
	seedReplyStateMsg(t, s, "plain", "inbox-1", "plain@x", "", "Hello", 1, now)
	// Server says it was answered.
	seedReplyStateMsg(t, s, "flagged", "inbox-1", "flagged@x", "", "Flagged", 2, now)
	if _, err := db.Exec(`UPDATE messages SET is_answered = 1 WHERE id = 'flagged'`); err != nil {
		t.Fatalf("flag: %v", err)
	}
	// No flag, but Sent holds a reply (bracketed In-Reply-To).
	seedReplyStateMsg(t, s, "viaSent", "inbox-1", "viasent@x", "", "Question", 3, now)
	seedReplyStateMsg(t, s, "sentReply", "sent-1", "r1@x", "<viasent@x>", "Re: Question", 4, now)
	// No flag, but Sent holds a forward.
	seedReplyStateMsg(t, s, "fwd", "inbox-1", "fwd@x", "", "Invoice", 5, now)
	seedReplyStateMsg(t, s, "sentFwd", "sent-1", "r2@x", "fwd@x", "Fwd: Invoice", 6, now)

	got := replyStateByThread(t, s)
	check := func(thread string, answered, forwarded bool) {
		t.Helper()
		c := got[thread]
		if c == nil {
			t.Fatalf("conversation %s missing", thread)
		}
		if c.IsAnswered != answered || c.IsForwarded != forwarded {
			t.Errorf("%s: answered=%v forwarded=%v, want %v/%v", thread, c.IsAnswered, c.IsForwarded, answered, forwarded)
		}
	}
	check("plain@x", false, false)
	check("flagged@x", true, false)
	check("viasent@x", true, false)
	check("fwd@x", false, true)
}

// A newer, unanswered message in the same thread clears the arrow.
func TestConversationReplyStateFollowsNewestMessage(t *testing.T) {
	s, db := newExpandCopiesTestStore(t)
	now := time.Now().UTC()

	seedReplyStateMsg(t, s, "first", "inbox-1", "t@x", "", "Topic", 1, now.Add(-2*time.Hour))
	seedReplyStateMsg(t, s, "myReply", "sent-1", "mine@x", "t@x", "Re: Topic", 2, now.Add(-time.Hour))
	if got := replyStateByThread(t, s)["t@x"]; got == nil || !got.IsAnswered {
		t.Fatalf("thread should be answered before the follow-up arrives: %+v", got)
	}

	seedReplyStateMsg(t, s, "followUp", "inbox-1", "t2@x", "mine@x", "Re: Topic", 3, now)
	if _, err := db.Exec(`UPDATE messages SET thread_id = 't@x' WHERE id = 'followUp'`); err != nil {
		t.Fatalf("thread: %v", err)
	}
	if got := replyStateByThread(t, s)["t@x"]; got == nil || got.IsAnswered {
		t.Fatalf("thread should not be answered once a newer message arrived: %+v", got)
	}
}

func TestMarkResponded(t *testing.T) {
	s, _ := newExpandCopiesTestStore(t)
	now := time.Now().UTC()
	seedReplyStateMsg(t, s, "orig", "inbox-1", "orig@x", "", "Hi", 1, now)
	seedReplyStateMsg(t, s, "sentCopy", "sent-1", "orig@x", "", "Hi", 2, now)

	msgs, err := s.MarkResponded("acct-1", "<orig@x>", false)
	if err != nil {
		t.Fatalf("MarkResponded: %v", err)
	}
	if len(msgs) != 1 || msgs[0].ID != "orig" || !msgs[0].IsAnswered || msgs[0].IsForwarded {
		t.Fatalf("unexpected result: %+v", msgs)
	}

	msgs, err = s.MarkResponded("acct-1", "orig@x", true)
	if err != nil {
		t.Fatalf("MarkResponded forward: %v", err)
	}
	if len(msgs) != 1 || !msgs[0].IsForwarded {
		t.Fatalf("forward flag not set: %+v", msgs)
	}

	if msgs, _ := s.MarkResponded("acct-1", "", false); msgs != nil {
		t.Fatalf("empty Message-ID should match nothing: %+v", msgs)
	}
}
