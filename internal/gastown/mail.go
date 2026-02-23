package gastown

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// MailMessage represents a message from gt mail inbox --json.
type MailMessage struct {
	ID       string `json:"id"`
	From     string `json:"from"`
	To       string `json:"to,omitempty"`
	Subject  string `json:"subject"`
	Body     string `json:"body,omitempty"`
	Time     string `json:"timestamp,omitempty"`
	Read     bool   `json:"read"`
	Priority string `json:"priority,omitempty"`
	Type     string `json:"type,omitempty"`
	ThreadID string `json:"thread_id,omitempty"`
	Pinned   bool   `json:"pinned,omitempty"`
}

// MailInbox fetches inbox messages via `gt mail inbox --json`.
// If unreadOnly is true, only unread messages are returned.
func MailInbox(unreadOnly bool) ([]MailMessage, error) {
	args := []string{"mail", "inbox", "--json"}
	if unreadOnly {
		args = append(args, "--unread")
	}
	out, err := exec.Command("gt", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("gt mail inbox: %w", err)
	}
	var msgs []MailMessage
	if err := json.Unmarshal(out, &msgs); err != nil {
		return nil, fmt.Errorf("gt mail inbox parse: %w", err)
	}
	return msgs, nil
}

// MailRead fetches a single message by ID via `gt mail read <id> --json`.
func MailRead(messageID string) (*MailMessage, error) {
	out, err := exec.Command("gt", "mail", "read", messageID, "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("gt mail read: %w", err)
	}
	var msg MailMessage
	if err := json.Unmarshal(out, &msg); err != nil {
		return nil, fmt.Errorf("gt mail read parse: %w", err)
	}
	return &msg, nil
}

// MailReply replies to a message via `gt mail reply <id> -m <body>`.
func MailReply(messageID, body string) error {
	out, err := exec.Command("gt", "mail", "reply", messageID, "-m", body).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gt mail reply: %w (%s)", err, string(out))
	}
	return nil
}

// MailSend sends a new message via `gt mail send <address> -s <subject> -m <body>`.
func MailSend(address, subject, body string) error {
	out, err := exec.Command("gt", "mail", "send", address, "-s", subject, "-m", body).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gt mail send: %w (%s)", err, string(out))
	}
	return nil
}

// MailArchive archives a message via `gt mail archive <id>`.
func MailArchive(messageID string) error {
	out, err := exec.Command("gt", "mail", "archive", messageID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gt mail archive: %w (%s)", err, string(out))
	}
	return nil
}

// MailMarkRead marks a message as read via `gt mail mark-read <id>`.
func MailMarkRead(messageID string) error {
	out, err := exec.Command("gt", "mail", "mark-read", messageID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gt mail mark-read: %w (%s)", err, string(out))
	}
	return nil
}
