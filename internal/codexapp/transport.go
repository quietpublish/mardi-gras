package codexapp

import (
	"errors"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// SubprocessTransport runs `codex app-server` as a child process and proxies
// stdio for the Client. Stderr is captured into Logs for inclusion in error
// messages; it is intentionally not echoed to mg's terminal since the parent
// is a fullscreen TUI.
type SubprocessTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	stderrBuf chan string
	stderrWG  sync.WaitGroup

	closeOnce sync.Once
	closed    bool
	closeMu   sync.Mutex
}

// SubprocessOption customizes SubprocessTransport construction.
type SubprocessOption func(*subprocessOptions)

type subprocessOptions struct {
	binary string
	args   []string
	dir    string
	env    []string
}

// WithBinary overrides the codex binary path. Defaults to "codex".
func WithBinary(path string) SubprocessOption {
	return func(o *subprocessOptions) { o.binary = path }
}

// WithExtraArgs appends arguments to `codex app-server`.
func WithExtraArgs(args ...string) SubprocessOption {
	return func(o *subprocessOptions) { o.args = append(o.args, args...) }
}

// WithDir sets the working directory of the subprocess.
func WithDir(dir string) SubprocessOption {
	return func(o *subprocessOptions) { o.dir = dir }
}

// WithEnv supplies an environment for the subprocess. If empty, os.Environ()
// is used.
func WithEnv(env []string) SubprocessOption {
	return func(o *subprocessOptions) { o.env = env }
}

// SpawnSubprocess launches `codex app-server` and returns a Transport wired
// to its stdio.
func SpawnSubprocess(opts ...SubprocessOption) (*SubprocessTransport, error) {
	o := subprocessOptions{
		binary: "codex",
		args:   []string{"app-server"},
	}
	for _, opt := range opts {
		opt(&o)
	}

	cmd := exec.Command(o.binary, o.args...) //nolint:gosec // binary is a configured codex path, not user input
	if o.dir != "" {
		cmd.Dir = o.dir
	}
	if len(o.env) > 0 {
		cmd.Env = o.env
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, err
	}

	t := &SubprocessTransport{
		cmd:       cmd,
		stdin:     stdin,
		stdout:    stdout,
		stderr:    stderr,
		stderrBuf: make(chan string, 64),
	}
	t.stderrWG.Add(1)
	go t.drainStderr()
	return t, nil
}

// Reader implements Transport.
func (s *SubprocessTransport) Reader() io.Reader { return s.stdout }

// Writer implements Transport.
func (s *SubprocessTransport) Writer() io.Writer { return s.stdin }

// Close terminates the subprocess. It first closes stdin (giving codex a
// chance to exit cleanly), then waits up to 2 seconds, then SIGTERMs, then
// SIGKILLs. Idempotent.
func (s *SubprocessTransport) Close() error {
	var err error
	s.closeOnce.Do(func() {
		s.closeMu.Lock()
		s.closed = true
		s.closeMu.Unlock()
		_ = s.stdin.Close()

		done := make(chan error, 1)
		go func() { done <- s.cmd.Wait() }()

		select {
		case err = <-done:
		case <-time.After(2 * time.Second):
			_ = s.cmd.Process.Signal(syscall.SIGTERM)
			select {
			case err = <-done:
			case <-time.After(1 * time.Second):
				_ = s.cmd.Process.Kill()
				err = <-done
			}
		}

		_ = s.stdout.Close()
		_ = s.stderr.Close()
		s.stderrWG.Wait()
		close(s.stderrBuf)

		// Treat expected shutdown errors as success.
		if err != nil {
			if ee := (&exec.ExitError{}); errors.As(err, &ee) {
				// Subprocess exited non-zero — fine if we initiated shutdown.
				err = nil
			}
		}
	})
	return err
}

// StderrLines returns up to limit recent stderr lines. Useful for crash
// diagnostics.
func (s *SubprocessTransport) StderrLines(limit int) []string {
	var lines []string
	for {
		select {
		case line, ok := <-s.stderrBuf:
			if !ok {
				return lines
			}
			lines = append(lines, line)
			if len(lines) >= limit {
				return lines
			}
		default:
			return lines
		}
	}
}

func (s *SubprocessTransport) drainStderr() {
	defer s.stderrWG.Done()
	buf := make([]byte, 4096)
	var carry []byte
	for {
		n, err := s.stderr.Read(buf)
		if n > 0 {
			carry = append(carry, buf[:n]...)
			data := carry
			carry = nil
			start := 0
			for i, b := range data {
				if b == '\n' {
					line := string(data[start:i])
					select {
					case s.stderrBuf <- line:
					default:
						// Drop oldest by draining one slot.
						<-s.stderrBuf
						s.stderrBuf <- line
					}
					start = i + 1
				}
			}
			if start < len(data) {
				carry = append(carry, data[start:]...)
			}
		}
		if err != nil {
			return
		}
	}
}
