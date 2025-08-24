// Package testutil provides testing utilities for the pipeliner application
package testutil

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// MockCommandRunner implements tools.CommandRunner for testing
type MockCommandRunner struct {
	mu        sync.RWMutex
	commands  []ExecutedCommand
	responses map[string]CommandResponse
}

type ExecutedCommand struct {
	Command string
	Args    []string
	Context context.Context
}

type CommandResponse struct {
	Error error
	Delay time.Duration
}

func NewMockCommandRunner() *MockCommandRunner {
	return &MockCommandRunner{
		responses: make(map[string]CommandResponse),
	}
}

func (m *MockCommandRunner) Run(ctx context.Context, command string, args []string) error {
	m.mu.Lock()
	m.commands = append(m.commands, ExecutedCommand{
		Command: command,
		Args:    args,
		Context: ctx,
	})
	m.mu.Unlock()

	key := command + " " + strings.Join(args, " ")

	m.mu.RLock()
	response, exists := m.responses[key]
	m.mu.RUnlock()

	if exists {
		if response.Delay > 0 {
			time.Sleep(response.Delay)
		}
		return response.Error
	}

	return nil
}

func (m *MockCommandRunner) SetResponse(command string, args []string, response CommandResponse) {
	key := command + " " + strings.Join(args, " ")
	m.mu.Lock()
	m.responses[key] = response
	m.mu.Unlock()
}

func (m *MockCommandRunner) GetExecutedCommands() []ExecutedCommand {
	m.mu.RLock()
	defer m.mu.RUnlock()

	commands := make([]ExecutedCommand, len(m.commands))
	copy(commands, m.commands)
	return commands
}

func (m *MockCommandRunner) Reset() {
	m.mu.Lock()
	m.commands = nil
	m.responses = make(map[string]CommandResponse)
	m.mu.Unlock()
}

// TempDir creates a temporary directory for testing and returns a cleanup function
func TempDir(t *testing.T, prefix string) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	cleanup := func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("Failed to clean up temp dir %s: %v", dir, err)
		}
	}

	return dir, cleanup
}

// CreateTestFile creates a test file with the given content
func CreateTestFile(t *testing.T, dir, filename, content string) string {
	t.Helper()

	filePath := filepath.Join(dir, filename)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file %s: %v", filePath, err)
	}

	return filePath
}

// CaptureOutput captures stdout/stderr during test execution
func CaptureOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	// Create pipes for stdout and stderr
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}
	defer stdoutR.Close()
	defer stdoutW.Close()

	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create stderr pipe: %v", err)
	}
	defer stderrR.Close()
	defer stderrW.Close()

	// Save original stdout/stderr
	origStdout := os.Stdout
	origStderr := os.Stderr

	// Replace stdout/stderr with pipes
	os.Stdout = stdoutW
	os.Stderr = stderrW

	// Channel to receive output
	stdoutC := make(chan string, 1)
	stderrC := make(chan string, 1)

	// Start goroutines to read from pipes
	go func() {
		defer close(stdoutC)
		buf, err := io.ReadAll(stdoutR)
		if err != nil {
			t.Errorf("Failed to read stdout: %v", err)
			return
		}
		stdoutC <- string(buf)
	}()

	go func() {
		defer close(stderrC)
		buf, err := io.ReadAll(stderrR)
		if err != nil {
			t.Errorf("Failed to read stderr: %v", err)
			return
		}
		stderrC <- string(buf)
	}()

	// Execute function
	fn()

	// Restore original stdout/stderr
	os.Stdout = origStdout
	os.Stderr = origStderr

	// Close writers to signal EOF to readers
	stdoutW.Close()
	stderrW.Close()

	// Wait for output
	stdout := <-stdoutC
	stderr := <-stderrC

	return stdout, stderr
}

// WithTimeout creates a context with timeout for tests
func WithTimeout(t *testing.T, timeout time.Duration) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), timeout)
}

// AssertNoError asserts that the error is nil
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

// AssertError asserts that an error occurred
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

// AssertEquals asserts that two values are equal
func AssertEquals(t *testing.T, expected, actual interface{}) {
	t.Helper()
	if expected != actual {
		t.Fatalf("Expected %v, got %v", expected, actual)
	}
}
