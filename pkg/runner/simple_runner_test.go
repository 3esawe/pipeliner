package runner_test

import (
	"context"
	"testing"

	"pipeliner/pkg/runner"
	"pipeliner/pkg/tools"
)

func TestSimpleRunner_Run(t *testing.T) {
	simpleRunner := runner.NewSimpleRunner()

	// Test basic command execution
	ctx := context.Background()
	err := simpleRunner.Run(ctx, "echo", []string{"test"})
	if err != nil {
		t.Fatalf("SimpleRunner.Run failed: %v", err)
	}

	// Verify Windows-style paths are accepted as arguments
	err = simpleRunner.Run(ctx, "echo", []string{"C:\\temp\\wordlist.txt"})
	if err != nil {
		t.Fatalf("SimpleRunner.Run rejected Windows path argument: %v", err)
	}
}

func TestSimpleRunner_InterpreterResolution(t *testing.T) {
	simpleRunner := runner.NewSimpleRunner()

	testCases := []struct {
		name        string
		command     string
		args        []string
		expectError bool
		description string
	}{
		{
			name:        "regular command - echo",
			command:     "echo",
			args:        []string{"hello"},
			expectError: false,
			description: "Should execute echo command successfully",
		},
		{
			name:        "nonexistent python script",
			command:     "nonexistent_script.py",
			args:        []string{"arg1"},
			expectError: true,
			description: "Should fail for nonexistent Python script but not panic",
		},
		{
			name:        "shell script extension",
			command:     "nonexistent_script.sh",
			args:        []string{},
			expectError: true,
			description: "Should attempt to use sh interpreter for .sh files",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			err := simpleRunner.Run(ctx, tc.command, tc.args)

			if tc.expectError && err == nil {
				t.Errorf("Expected error for %s, but got none", tc.description)
			}

			if !tc.expectError && err != nil {
				t.Errorf("Expected success for %s, but got error: %v", tc.description, err)
			}
		})
	}
}

func TestSimpleRunner_ImplementsInterface(t *testing.T) {
	// Verify that SimpleRunner implements tools.CommandRunner interface
	var _ tools.CommandRunner = (*runner.SimpleRunner)(nil)

	// Test that it can be used with ReplacementCommandRunner
	simpleRunner := runner.NewSimpleRunner()
	replacementRunner := runner.NewReplacementCommandRunner(simpleRunner)

	if replacementRunner == nil {
		t.Fatal("Failed to create ReplacementCommandRunner with SimpleRunner")
	}

	// Test basic execution through replacement runner
	ctx := context.Background()
	err := replacementRunner.Run(ctx, "echo", []string{"interface test"})
	if err != nil {
		t.Fatalf("Failed to execute command through ReplacementCommandRunner: %v", err)
	}
}
