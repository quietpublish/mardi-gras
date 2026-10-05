// Package codexapp speaks Codex's app-server protocol (JSON-RPC 2.0 over the
// stdio of `codex app-server`) to run an in-app Codex session: start a thread,
// run turns on it, stream what the agent does, and answer its approval
// requests. Codex removed `codex mcp-server`, which mg used before, in 0.154.0
// (mg-xge.2); app-server is the supported integration point (0.115.0+ for the
// per-thread approvals reviewer).
//
// The client translates app-server notifications into the CodexEvent shapes
// the transcript renders (agent_message, exec_command_begin/end, …), so the
// views do not depend on the wire protocol. Only the subset of the protocol
// mg needs is modeled; see docs/internal/codex-app-server-protocol.md.
package codexapp

import (
	"encoding/json"
	"strings"
)

// JSON-RPC envelope types. app-server frames newline-delimited JSON-RPC 2.0;
// it omits "jsonrpc" on what it sends and accepts it either way.

// request is a client → server JSON-RPC request.
type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// notification is a JSON-RPC notification (no id).
type notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// response is any inbound message: a response to one of our requests (id, no
// method), a server-initiated request (id and method), or a notification
// (method, no id). readLoop tells them apart.
type response struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// parseIntID decodes a numeric JSON-RPC id; our own requests use ints.
func parseIntID(raw json.RawMessage) (int, bool) {
	var id int
	if err := json.Unmarshal(raw, &id); err != nil {
		return 0, false
	}
	return id, true
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	methodInitialize    = "initialize"
	methodInitialized   = "initialized"
	methodThreadStart   = "thread/start"
	methodTurnStart     = "turn/start"
	methodTurnInterrupt = "turn/interrupt"

	clientName            = "mardi-gras"
	clientVersionFallback = "dev"
	jsonRPCVersion        = "2.0"

	// rpcMethodNotFound answers a server request mg does not handle.
	rpcMethodNotFound = -32601
)

// optOutNotifications are chatty notifications mg does not render; asking
// the server not to send them keeps the read loop and event buffers quiet.
var optOutNotifications = []string{
	"item/agentMessage/delta",
	"item/reasoning/summaryTextDelta",
	"item/reasoning/summaryPartAdded",
	"item/reasoning/textDelta",
	"item/commandExecution/outputDelta",
	"account/rateLimits/updated",
	"thread/tokenUsage/updated",
	"remoteControl/status/changed",
}

// CodexEvent is one event for the transcript, in the shape mg rendered from
// codex's MCP stream: Msg is a JSON object whose "type" names the variant.
// The client builds these from app-server notifications (see translate).
type CodexEvent struct {
	Meta EventMeta       `json:"_meta"`
	Msg  json.RawMessage `json:"msg"`
}

// EventMeta carries the thread an event belongs to.
type EventMeta struct {
	ThreadID string `json:"threadId"`
}

// EventType returns the `msg.type` discriminator, or "" when Msg is empty or
// malformed.
func (e CodexEvent) EventType() string {
	if len(e.Msg) == 0 {
		return ""
	}
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(e.Msg, &probe); err != nil {
		return ""
	}
	return probe.Type
}

// Typed event payloads, as the transcript reads them.

// AgentMessageEvent is `type == "agent_message"`: the assistant's text.
type AgentMessageEvent struct {
	Message string `json:"message"`
}

// UserMessageEvent is `type == "user_message"`: the prompt sent in.
type UserMessageEvent struct {
	Message string `json:"message"`
}

// ExecCommandBeginEvent is `type == "exec_command_begin"`.
type ExecCommandBeginEvent struct {
	CallID  string   `json:"call_id"`
	Command []string `json:"command"`
	Cwd     string   `json:"cwd"`
}

// ExecCommandEndEvent is `type == "exec_command_end"`. Status is the item's
// final status ("completed", "failed", "declined"); a declined or failed
// command has no exit code.
type ExecCommandEndEvent struct {
	CallID   string `json:"call_id"`
	ExitCode *int   `json:"exit_code"`
	Status   string `json:"status"`
}

// MCPToolCallBeginEvent is `type == "mcp_tool_call_begin"`.
type MCPToolCallBeginEvent struct {
	CallID     string `json:"call_id"`
	Invocation struct {
		Server string `json:"server"`
		Tool   string `json:"tool"`
	} `json:"invocation"`
}

// MCPToolCallEndEvent is `type == "mcp_tool_call_end"`.
type MCPToolCallEndEvent struct {
	CallID  string `json:"call_id"`
	IsError bool   `json:"is_error"`
}

// PatchEvent is `type == "patch"`: a file change the agent made or proposes.
type PatchEvent struct {
	CallID string   `json:"call_id"`
	Paths  []string `json:"paths"`
	Status string   `json:"status"`
}

// TaskStartedEvent is `type == "task_started"`: a turn began.
type TaskStartedEvent struct {
	TurnID string `json:"turn_id"`
}

// TaskCompleteEvent is `type == "task_complete"`: a turn ended.
type TaskCompleteEvent struct {
	LastAgentMessage string `json:"last_agent_message"`
	Status           string `json:"status"`
}

// ErrorEvent is `type == "error"`.
type ErrorEvent struct {
	Message string `json:"message"`
}

// SessionConfiguredEvent is `type == "session_configured"`: the thread is up,
// with the settings the server actually applied.
type SessionConfiguredEvent struct {
	ThreadID       string `json:"thread_id"`
	Model          string `json:"model"`
	ApprovalPolicy string `json:"approval_policy"`
	Cwd            string `json:"cwd"`
}

// RequestResolvedEvent is `type == "server_request_resolved"`: the server no
// longer waits on the approval request with this id (answered, or the turn was
// interrupted). An open modal for it must close.
type RequestResolvedEvent struct {
	RequestID json.RawMessage `json:"request_id"`
}

// ServerRequest is an approval request from the server. RawID must be echoed
// back verbatim on the reply (see Client.Respond); the server picks its type.
// Approval is already parsed; the client answers every other kind of server
// request itself.
type ServerRequest struct {
	RawID    json.RawMessage
	Method   string
	Params   json.RawMessage
	Approval Approval
}

// Approval is a command or file-change approval, normalized for mg's modal,
// deny-list and Jev reading.
type Approval struct {
	Kind    string                     // "exec" | "patch"
	Message string                     // the agent's reason, if any
	Command []string                   // exec: argv (split from the shell string)
	Cwd     string                     // exec: working directory
	Reason  string                     // why approval is needed
	Changes map[string]json.RawMessage // patch: path -> change
	Root    string                     // patch: a directory the agent asks to write under
}

// Approval request methods mg surfaces as a modal.
const (
	methodCommandApproval    = "item/commandExecution/requestApproval"
	methodFileChangeApproval = "item/fileChange/requestApproval"
)

// SameRequestID reports whether two raw JSON-RPC ids are the same id: 7, "7"
// and " 7 " are compared by their decoded value.
func SameRequestID(a, b json.RawMessage) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return strings.TrimSpace(string(a)) == strings.TrimSpace(string(b))
	}
	return x == y
}

// splitShell splits a POSIX shell command string into argv: whitespace
// separates words, single quotes are literal, double quotes allow \" and \\,
// a backslash escapes the next character outside quotes. app-server sends a
// command approval as one shlex-joined string; the deny-list and the Jev
// reading work on argv. ok is false on an unterminated quote, and callers
// fall back to the whole string as a single word, which the deny-list still
// scans (fail closed).
func splitShell(s string) (argv []string, ok bool) {
	var (
		cur     strings.Builder
		inWord  bool
		quote   rune
		escaped bool
	)
	flush := func() {
		if inWord {
			argv = append(argv, cur.String())
			cur.Reset()
			inWord = false
		}
	}
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case quote == '\'':
			if r == '\'' {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case quote == '"':
			switch r {
			case '"':
				quote = 0
			case '\\':
				escaped = true
			default:
				cur.WriteRune(r)
			}
		case r == '\\':
			escaped, inWord = true, true
		case r == '\'' || r == '"':
			quote, inWord = r, true
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if quote != 0 || escaped {
		return nil, false
	}
	flush()
	return argv, true
}
