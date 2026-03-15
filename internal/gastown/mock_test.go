package gastown

import "time"

// mockRun replaces runWithTimeout with a stub returning fixed output.
func mockRun(output []byte, err error) func() {
	orig := runWithTimeout
	runWithTimeout = func(_ time.Duration, _ string, _ ...string) ([]byte, error) {
		return output, err
	}
	return func() { runWithTimeout = orig }
}

// mockRunCapture replaces runWithTimeout, capturing all calls.
func mockRunCapture(output []byte, err error) (calls *[][]string, restore func()) {
	var c [][]string
	orig := runWithTimeout
	runWithTimeout = func(_ time.Duration, name string, args ...string) ([]byte, error) {
		c = append(c, append([]string{name}, args...))
		return output, err
	}
	return &c, func() { runWithTimeout = orig }
}

// mockCombined replaces runCombinedWithTimeout with a stub returning fixed output.
func mockCombined(output []byte, err error) func() {
	orig := runCombinedWithTimeout
	runCombinedWithTimeout = func(_ time.Duration, _ string, _ ...string) ([]byte, error) {
		return output, err
	}
	return func() { runCombinedWithTimeout = orig }
}

// mockCombinedCapture replaces runCombinedWithTimeout, capturing all calls.
func mockCombinedCapture(output []byte, err error) (calls *[][]string, restore func()) {
	var c [][]string
	orig := runCombinedWithTimeout
	runCombinedWithTimeout = func(_ time.Duration, name string, args ...string) ([]byte, error) {
		c = append(c, append([]string{name}, args...))
		return output, err
	}
	return &c, func() { runCombinedWithTimeout = orig }
}

// mockExecCapture replaces execWithTimeout, capturing all calls.
func mockExecCapture(err error) (calls *[][]string, restore func()) {
	var c [][]string
	orig := execWithTimeout
	execWithTimeout = func(_ time.Duration, name string, args ...string) error {
		c = append(c, append([]string{name}, args...))
		return err
	}
	return &c, func() { execWithTimeout = orig }
}
