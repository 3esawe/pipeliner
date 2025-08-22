package engine

import (
	"sync"
	"time"
)

// MockNotifier implements a DomainNotifier for testing
type MockNotifier struct {
	mu            sync.Mutex
	notifications []string
	received      chan struct{}
}

func NewMockNotifier() *MockNotifier {
	return &MockNotifier{
		received: make(chan struct{}, 10),
	}
}

func (m *MockNotifier) SendDomainAddedMessage(domain string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifications = append(m.notifications, domain)
	m.received <- struct{}{}
	return nil
}

func (m *MockNotifier) GetNotifications() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.notifications
}

func (m *MockNotifier) WaitForNotifications(count int, timeout time.Duration) bool {
	for i := 0; i < count; i++ {
		select {
		case <-m.received:
		case <-time.After(timeout):
			return false
		}
	}
	return true
}

// func TestNotificationSending(t *testing.T) {
// 	// Create mock notifier
// 	mockNotifier := NewMockNotifier()

// 	// Create engine with mock notifier
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()
// 	discord_client, _ := notification.NewNotificationClient()
// 	engine := NewPiplinerEngine(ctx,
// 		WithPeriodic(1),
// 		WithNotificationClient(discord_client),
// 	)

// 	// Create temp directory for test
// 	tempDir, err := os.MkdirTemp("", "pipeliner-test-")
// 	if err != nil {
// 		t.Fatalf("Failed to create temp dir: %v", err)
// 	}
// 	defer os.RemoveAll(tempDir)

// 	// Change to temp directory
// 	oldDir, _ := os.Getwd()
// 	defer os.Chdir(oldDir)
// 	if err := os.Chdir(tempDir); err != nil {
// 		t.Fatalf("Failed to change directory: %v", err)
// 	}

// 	// Create test domain file
// 	filePath := filepath.Join(tempDir, "test_domains.txt")
// 	content := `example.com
// sub.example.com
// new-domain.org
// # Comment line
// invalid-domain
// 192.168.1.1
// http://web.example.com
// https://secure.example.org:8443
// `
// 	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
// 		t.Fatalf("Failed to create test file: %v", err)
// 	}

// 	// Set initial known domains
// 	engine.knownDomainsMu.Lock()
// 	engine.knownDomains["example.com"] = true
// 	engine.knownDomainsMu.Unlock()
// 	engine.firstScanComplete = true

// 	// Process the file with notifications enabled
// 	engine.processDomainFile(filePath, true)
// 	// Wait for notifications (should get 2: sub.example.com and new-domain.org)
// 	if !mockNotifier.WaitForNotifications(4, 2*time.Second) {
// 		t.Fatal("Timed out waiting for notifications")
// 	}

// 	// Verify notifications
// 	notifications := mockNotifier.GetNotifications()
// 	expected := []string{"sub.example.com", "new-domain.org", "web.example.com", "secure.example.org"}

// 	if len(notifications) != len(expected) {
// 		t.Fatalf("Expected %d notifications, got %d", len(expected), len(notifications))
// 	}

// 	for _, domain := range expected {
// 		found := false
// 		for _, n := range notifications {
// 			if n == domain {
// 				found = true
// 				break
// 			}
// 		}
// 		if !found {
// 			t.Errorf("Missing notification for domain: %s", domain)
// 		}
// 	}
// }

// func TestDomainValidation(t *testing.T) {
// 	testCases := []struct {
// 		domain string
// 		valid  bool
// 	}{
// 		{"example.com", true},
// 		{"sub.example.com", true},
// 		{"new-domain.org", true},
// 		{"invalid-domain", false}, // No dot
// 		{"example.com", true},     // Spaces will be trimmed
// 		// IP address
// 	}

// 	for _, tc := range testCases {
// 		actual := isValidDomain(tc.domain)
// 		if actual != tc.valid {
// 			t.Errorf("Validation mismatch for '%s': expected %v, got %v",
// 				tc.domain, tc.valid, actual)
// 		}
// 	}
// }
