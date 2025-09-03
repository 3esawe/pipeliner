package runner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"pipeliner/pkg/runner"
)

// MockBaseRunner implements the CommandRunner interface for testing
type MockBaseRunner struct {
	ExecutedCommands []ExecutedCommand
}

type ExecutedCommand struct {
	Command string
	Args    []string
}

func (m *MockBaseRunner) Run(ctx context.Context, command string, args []string) error {
	m.ExecutedCommands = append(m.ExecutedCommands, ExecutedCommand{
		Command: command,
		Args:    args,
	})
	return nil
}

func TestReplacementCommandRunner_RunWithReplacement(t *testing.T) {
	// Create a temporary file with test URLs
	tempDir, err := os.MkdirTemp("", "test_replacement_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	urlFile := filepath.Join(tempDir, "urls.txt")
	urlContent := `http://example.com
https://test.com
http://demo.org`

	err = os.WriteFile(urlFile, []byte(urlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create mock base runner
	mockRunner := &MockBaseRunner{}

	// Create replacement runner
	replacementRunner := runner.NewReplacementCommandRunner(mockRunner)

	// Test replacement execution
	ctx := context.Background()
	command := "ffuf"
	args := []string{"-u", "{{URL}}/FUZZ", "-w", "/path/to/wordlist.txt", "-mc", "200,401-403"}
	replaceToken := "{{URL}}"

	err = replacementRunner.RunWithReplacement(ctx, command, args, replaceToken, urlFile)
	if err != nil {
		t.Fatalf("RunWithReplacement failed: %v", err)
	}

	// Verify that the command was executed for each URL
	expectedCommands := 3
	if len(mockRunner.ExecutedCommands) != expectedCommands {
		t.Fatalf("Expected %d commands, got %d", expectedCommands, len(mockRunner.ExecutedCommands))
	}

	// Check that URLs were properly replaced
	expectedURLs := []string{"http://example.com", "https://test.com", "http://demo.org"}
	for i, execCmd := range mockRunner.ExecutedCommands {
		if execCmd.Command != "ffuf" {
			t.Errorf("Expected command 'ffuf', got '%s'", execCmd.Command)
		}

		expectedArg := expectedURLs[i] + "/FUZZ"
		if execCmd.Args[1] != expectedArg {
			t.Errorf("Expected URL argument '%s', got '%s'", expectedArg, execCmd.Args[1])
		}
	}
}

func TestReplacementCommandRunner_Run(t *testing.T) {
	// Test normal execution without replacement
	mockRunner := &MockBaseRunner{}
	replacementRunner := runner.NewReplacementCommandRunner(mockRunner)

	ctx := context.Background()
	command := "httpx"
	args := []string{"-l", "input.txt", "-o", "output.txt"}

	err := replacementRunner.Run(ctx, command, args)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(mockRunner.ExecutedCommands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(mockRunner.ExecutedCommands))
	}

	execCmd := mockRunner.ExecutedCommands[0]
	if execCmd.Command != command {
		t.Errorf("Expected command '%s', got '%s'", command, execCmd.Command)
	}

	if len(execCmd.Args) != len(args) {
		t.Errorf("Expected %d args, got %d", len(args), len(execCmd.Args))
	}

	for i, expectedArg := range args {
		if execCmd.Args[i] != expectedArg {
			t.Errorf("Expected arg '%s', got '%s'", expectedArg, execCmd.Args[i])
		}
	}
}
