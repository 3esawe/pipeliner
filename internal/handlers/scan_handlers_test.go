package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"pipeliner/internal/models"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockScanService is a mock implementation of the ScanService interface
// This allows us to control the behavior of dependencies during testing
type MockScanService struct {
	mock.Mock // Embed testify/mock to get mocking capabilities
}

// StartScan mocks the StartScan method
// The mock.Called() records the method call and arguments
// The Return() specifies what values to return
func (m *MockScanService) StartScan(scan *models.Scan) (string, error) {
	args := m.Called(scan)               // Record that this method was called with these args
	return args.String(0), args.Error(1) // Return the mocked values
}

// GetScanByUUID mocks the GetScanByUUID method
func (m *MockScanService) GetScanByUUID(id string) (*models.Scan, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Scan), args.Error(1)
}

// TestStartScan tests the StartScan handler function
func TestStartScan(t *testing.T) {
	// Set Gin to test mode to reduce noise in test output
	gin.SetMode(gin.TestMode)

	// Define test cases using table-driven testing pattern
	// This allows us to test multiple scenarios with the same test logic
	tests := []struct {
		name           string                             // Test case name for clarity
		requestBody    string                             // JSON payload to send
		setupMock      func(*MockScanService)             // Function to configure mock behavior
		expectedStatus int                                // Expected HTTP status code
		expectedBody   string                             // Expected response body
		validateMock   func(*testing.T, *MockScanService) // Optional: additional mock validations
	}{
		{
			name:        "Valid Request - Success",
			requestBody: `{"scan_type":"subdomain_alive","domain":"example.com"}`,
			setupMock: func(m *MockScanService) {
				// Configure mock to return success when StartScan is called
				m.On("StartScan", mock.MatchedBy(func(scan *models.Scan) bool {
					// Verify the scan object has expected values
					return scan.ScanType == "subdomain_alive" &&
						scan.Domain == "example.com"
				})).Return("123e4567-e89b-12d3-a456-426614174000", nil)
			},
			expectedStatus: 200,
			expectedBody:   `{"scan_id":"123e4567-e89b-12d3-a456-426614174000"}`,
			validateMock: func(t *testing.T, m *MockScanService) {
				// Verify StartScan was called exactly once
				m.AssertNumberOfCalls(t, "StartScan", 1)
			},
		},
		{
			name:        "Invalid JSON - Malformed",
			requestBody: `{"scan_type":"subdomain_alive","domain":}`, // Missing value
			setupMock: func(m *MockScanService) {
				// No mock setup needed - handler should fail before calling service
			},
			expectedStatus: 400,
			expectedBody:   `{"error":"Invalid request payload"}`,
			validateMock: func(t *testing.T, m *MockScanService) {
				// Verify StartScan was never called due to JSON parsing failure
				m.AssertNumberOfCalls(t, "StartScan", 0)
			},
		},
		{
			name:        "Missing Required Field - scan_type",
			requestBody: `{"domain":"example.com"}`, // Missing scan_type
			setupMock: func(m *MockScanService) {
				// No mock setup needed
			},
			expectedStatus: 400,
			expectedBody:   `{"error":"Invalid request payload"}`,
		},
		{
			name:        "Missing Required Field - domain",
			requestBody: `{"scan_type":"subdomain_alive"}`, // Missing domain
			setupMock: func(m *MockScanService) {
				// No mock setup needed
			},
			expectedStatus: 400,
			expectedBody:   `{"error":"Invalid request payload"}`,
		},
		{
			name:        "Service Error - Internal Error",
			requestBody: `{"scan_type":"subdomain_alive","domain":"example.com"}`,
			setupMock: func(m *MockScanService) {
				// Configure mock to return an error
				m.On("StartScan", mock.AnythingOfType("*models.Scan")).
					Return("", errors.New("database connection failed"))
			},
			expectedStatus: 500,
			expectedBody:   `{"error":"Failed to start scan"}`,
		},
		{
			name:        "Empty Request Body",
			requestBody: `{}`,
			setupMock: func(m *MockScanService) {
				// No mock setup needed
			},
			expectedStatus: 400,
			expectedBody:   `{"error":"Invalid request payload"}`,
		},
	}

	// Run each test case
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new mock for each test to avoid interference
			mockService := new(MockScanService)

			// Setup mock behavior for this specific test
			tt.setupMock(mockService)

			// Create handler with mock service
			handler := NewScanHandler(mockService)

			// Setup Gin router for this test
			router := gin.New() // Use gin.New() instead of Default() to avoid middleware
			router.POST("/api/scans", handler.StartScan)

			// Create HTTP request
			req, err := http.NewRequest("POST", "/api/scans", strings.NewReader(tt.requestBody))
			assert.NoError(t, err, "Failed to create request")
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder to capture handler response
			w := httptest.NewRecorder()

			// Execute the request
			router.ServeHTTP(w, req)

			// Assert HTTP status code
			assert.Equal(t, tt.expectedStatus, w.Code,
				"Expected status %d, got %d. Response: %s",
				tt.expectedStatus, w.Code, w.Body.String())

			// Assert response body
			assert.JSONEq(t, tt.expectedBody, w.Body.String(),
				"Response body doesn't match expected JSON")

			// Run additional mock validations if provided
			if tt.validateMock != nil {
				tt.validateMock(t, mockService)
			}

			// Verify all expected mock calls were made
			mockService.AssertExpectations(t)
		})
	}
}

// TestGetScanByUUID tests the GetScanByUUID handler function
func TestGetScanByUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		scanID         string
		setupMock      func(*MockScanService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:   "Valid ID - Scan Found",
			scanID: "123e4567-e89b-12d3-a456-426614174000",
			setupMock: func(m *MockScanService) {
				scan := &models.Scan{
					UUID:     "123e4567-e89b-12d3-a456-426614174000",
					ScanType: "subdomain_alive",
					Domain:   "example.com",
					Status:   "running",
				}
				m.On("GetScanByUUID", "123e4567-e89b-12d3-a456-426614174000").
					Return(scan, nil)
			},
			expectedStatus: 200,
			// Note: This is a simplified expected body. In reality, you'd include all fields
		},
		{
			name:   "Valid ID - Scan Not Found",
			scanID: "non-existent-id",
			setupMock: func(m *MockScanService) {
				m.On("GetScanByUUID", "non-existent-id").
					Return(nil, errors.New("record not found"))
			},
			expectedStatus: 500,
			expectedBody:   `{"error":"Failed to get scan"}`,
		},
		{
			name:   "Service Returns Nil Scan",
			scanID: "some-id",
			setupMock: func(m *MockScanService) {
				m.On("GetScanByUUID", "some-id").
					Return((*models.Scan)(nil), nil) // Explicit nil pointer
			},
			expectedStatus: 404,
			expectedBody:   `{"error":"Scan not found"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockScanService)
			tt.setupMock(mockService)

			handler := NewScanHandler(mockService)
			router := gin.New()
			router.GET("/api/scans/:id", handler.GetScanByUUID)

			url := fmt.Sprintf("/api/scans/%s", tt.scanID)
			req, _ := http.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedBody != "" {
				assert.JSONEq(t, tt.expectedBody, w.Body.String())
			}

			mockService.AssertExpectations(t)
		})
	}
}

// Helper function to create a valid scan request body
func createScanRequestBody(scanType, domain string) string {
	req := ScanRequest{
		ScanType: scanType,
		Domain:   domain,
	}
	body, _ := json.Marshal(req)
	return string(body)
}

// Benchmark test to measure handler performance
func BenchmarkStartScan(b *testing.B) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockScanService)
	mockService.On("StartScan", mock.AnythingOfType("*models.Scan")).
		Return("test-id", nil)

	handler := NewScanHandler(mockService)
	router := gin.New()
	router.POST("/api/scans", handler.StartScan)

	requestBody := createScanRequestBody("subdomain_alive", "example.com")

	b.ResetTimer() // Don't count setup time

	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("POST", "/api/scans", strings.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}
