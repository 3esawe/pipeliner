package tools

import (
	"context"
	"testing"
	"time"

	"pipeliner/pkg/testutil"
)

// MockTool implements Tool for testing
type MockTool struct {
	name         string
	toolType     string
	dependencies []string
	runFunc      func(ctx context.Context, options *Options) error
	runCount     int
}

func NewMockTool(name, toolType string, dependencies []string) *MockTool {
	return &MockTool{
		name:         name,
		toolType:     toolType,
		dependencies: dependencies,
		runFunc:      func(context.Context, *Options) error { return nil },
	}
}

func (m *MockTool) Name() string        { return m.name }
func (m *MockTool) Type() string        { return m.toolType }
func (m *MockTool) DependsOn() []string { return m.dependencies }
func (m *MockTool) PostHooks() []string { return []string{} } // No hooks for mock tools

func (m *MockTool) Run(ctx context.Context, options *Options) error {
	m.runCount++
	return m.runFunc(ctx, options)
}

func (m *MockTool) SetRunFunc(fn func(ctx context.Context, options *Options) error) {
	m.runFunc = fn
}

func (m *MockTool) GetRunCount() int {
	return m.runCount
}

func TestSequentialStrategy_Run(t *testing.T) {
	// Create test context with timeout
	ctx, cancel := testutil.WithTimeout(t, 5*time.Second)
	defer cancel()

	// Create mock tools
	tool1 := NewMockTool("tool1", "test", nil)
	tool2 := NewMockTool("tool2", "test", nil)
	tools := []Tool{tool1, tool2}

	// Create options
	options := DefaultOptions()
	options.ScanType = "test"
	options.Domain = "example.com"

	// Test successful execution
	strategy := &SequentialStrategy{}
	err := strategy.Run(ctx, tools, options)

	testutil.AssertNoError(t, err)
	testutil.AssertEquals(t, 1, tool1.GetRunCount())
	testutil.AssertEquals(t, 1, tool2.GetRunCount())
}

func TestConcurrentStrategy_Run(t *testing.T) {
	ctx, cancel := testutil.WithTimeout(t, 5*time.Second)
	defer cancel()

	// Create mock tools with delays
	tool1 := NewMockTool("tool1", "test", nil)
	tool2 := NewMockTool("tool2", "test", nil)

	// Add small delays to test concurrency
	tool1.SetRunFunc(func(context.Context, *Options) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})
	tool2.SetRunFunc(func(context.Context, *Options) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	tools := []Tool{tool1, tool2}

	options := DefaultOptions()
	options.ScanType = "test"
	options.Domain = "example.com"

	// Test concurrent execution
	start := time.Now()
	strategy := &ConcurrentStrategy{}
	err := strategy.Run(ctx, tools, options)
	duration := time.Since(start)

	testutil.AssertNoError(t, err)
	testutil.AssertEquals(t, 1, tool1.GetRunCount())
	testutil.AssertEquals(t, 1, tool2.GetRunCount())

	// Should complete in less than 200ms (concurrent) rather than 200ms+ (sequential)
	if duration > 200*time.Millisecond {
		t.Errorf("Expected concurrent execution, but took %v", duration)
	}
}

func TestHybridStrategy_RunWithDependencies(t *testing.T) {
	ctx, cancel := testutil.WithTimeout(t, 5*time.Second)
	defer cancel()

	// Create tools with dependencies
	tool1 := NewMockTool("tool1", "test", nil)               // No dependencies
	tool2 := NewMockTool("tool2", "test", []string{"tool1"}) // Depends on tool1
	tool3 := NewMockTool("tool3", "test", []string{"tool1"}) // Depends on tool1

	tools := []Tool{tool1, tool2, tool3}

	options := DefaultOptions()
	options.ScanType = "test"
	options.Domain = "example.com"

	// Test hybrid execution with dependencies
	strategy := &HybridStrategy{}
	err := strategy.Run(ctx, tools, options)

	testutil.AssertNoError(t, err)
	testutil.AssertEquals(t, 1, tool1.GetRunCount())
	testutil.AssertEquals(t, 1, tool2.GetRunCount())
	testutil.AssertEquals(t, 1, tool3.GetRunCount())
}

func TestToolConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ToolConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: ToolConfig{
				Name:    "test-tool",
				Command: "echo",
				Type:    "test",
				Flags:   []FlagConfig{},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			config: ToolConfig{
				Command: "echo",
				Type:    "test",
			},
			wantErr: true,
		},
		{
			name: "missing command",
			config: ToolConfig{
				Name: "test-tool",
				Type: "test",
			},
			wantErr: true,
		},
		{
			name: "negative retries",
			config: ToolConfig{
				Name:    "test-tool",
				Command: "echo",
				Type:    "test",
				Retries: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				testutil.AssertError(t, err)
			} else {
				testutil.AssertNoError(t, err)
			}
		})
	}
}

func TestChainConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ChainConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: ChainConfig{
				ExecutionMode: "sequential",
				Tools: []ToolConfig{
					{Name: "tool1", Command: "echo", Type: "test"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid execution mode",
			config: ChainConfig{
				ExecutionMode: "invalid",
				Tools: []ToolConfig{
					{Name: "tool1", Command: "echo", Type: "test"},
				},
			},
			wantErr: true,
		},
		{
			name: "no tools",
			config: ChainConfig{
				ExecutionMode: "sequential",
				Tools:         []ToolConfig{},
			},
			wantErr: true,
		},
		{
			name: "duplicate tool names",
			config: ChainConfig{
				ExecutionMode: "sequential",
				Tools: []ToolConfig{
					{Name: "tool1", Command: "echo", Type: "test"},
					{Name: "tool1", Command: "echo", Type: "test"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				testutil.AssertError(t, err)
			} else {
				testutil.AssertNoError(t, err)
			}
		})
	}
}

func TestOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		options *Options
		wantErr bool
	}{
		{
			name: "valid options",
			options: &Options{
				ScanType: "test",
				Domain:   "example.com",
				Timeout:  time.Minute,
			},
			wantErr: false,
		},
		{
			name: "missing scan type",
			options: &Options{
				Domain:  "example.com",
				Timeout: time.Minute,
			},
			wantErr: true,
		},
		{
			name: "missing domain",
			options: &Options{
				ScanType: "test",
				Timeout:  time.Minute,
			},
			wantErr: true,
		},
		{
			name: "invalid timeout",
			options: &Options{
				ScanType: "test",
				Domain:   "example.com",
				Timeout:  -time.Second,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.options.Validate()
			if tt.wantErr {
				testutil.AssertError(t, err)
			} else {
				testutil.AssertNoError(t, err)
			}
		})
	}
}
