package gastown

import (
	"errors"
	"testing"
)

func TestMailInboxHappy(t *testing.T) {
	defer mockRun([]byte(gtMailInboxJSON), nil)()
	msgs, err := MailInbox(false)
	if err != nil {
		t.Fatalf("MailInbox() error = %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Subject != "New assignment" {
		t.Errorf("Subject = %q", msgs[0].Subject)
	}
}

func TestMailInboxEmpty(t *testing.T) {
	defer mockRun([]byte(`[]`), nil)()
	msgs, err := MailInbox(false)
	if err != nil {
		t.Fatalf("MailInbox() error = %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

func TestMailInboxUnreadOnlyFlag(t *testing.T) {
	calls, restore := mockRunCapture([]byte(`[]`), nil)
	defer restore()
	_, _ = MailInbox(true)
	if len(*calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(*calls))
	}
	args := (*calls)[0]
	// Should include --unread
	found := false
	for _, a := range args {
		if a == "--unread" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected --unread in args: %v", args)
	}
}

func TestMailInboxExecError(t *testing.T) {
	defer mockRun(nil, errors.New("timeout"))()
	_, err := MailInbox(false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMailReadHappy(t *testing.T) {
	defer mockRun([]byte(gtMailReadJSON), nil)()
	msg, err := MailRead("mail-001")
	if err != nil {
		t.Fatalf("MailRead() error = %v", err)
	}
	if msg.ID != "mail-001" {
		t.Errorf("ID = %q", msg.ID)
	}
	if msg.Body != "Please work on mg-10." {
		t.Errorf("Body = %q", msg.Body)
	}
}

func TestMailReadExecError(t *testing.T) {
	defer mockRun(nil, errors.New("not found"))()
	_, err := MailRead("mail-999")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMailReplyErrorWrapping(t *testing.T) {
	defer mockCombined([]byte("error details"), errors.New("exit 1"))()
	err := MailReply("mail-001", "thanks")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMailSendErrorWrapping(t *testing.T) {
	defer mockCombined([]byte("error details"), errors.New("exit 1"))()
	err := MailSend("polecat-nux", "hello", "body")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMailArchiveErrorWrapping(t *testing.T) {
	defer mockCombined([]byte("error details"), errors.New("exit 1"))()
	err := MailArchive("mail-001")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMailMarkReadErrorWrapping(t *testing.T) {
	defer mockCombined([]byte("error details"), errors.New("exit 1"))()
	err := MailMarkRead("mail-001")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
