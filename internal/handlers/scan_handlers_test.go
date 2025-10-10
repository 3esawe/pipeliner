package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"pipeliner/internal/models"
	"pipeliner/internal/services"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

type MockScanService struct {
	mock.Mock
}

func (m *MockScanService) StartScan(scan *models.Scan) (string, error) {
	args := m.Called(scan)
	return args.String(0), args.Error(1)
}

func (m *MockScanService) ListScans() ([]models.Scan, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Scan), args.Error(1)
}

func (m *MockScanService) GetScanByUUID(id string) (*models.Scan, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Scan), args.Error(1)
}

func (m *MockScanService) DeleteScan(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func TestStartScan(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		requestBody    string
		setupMock      func(*MockScanService)
		expectedStatus int
		expectedBody   string
		validateMock   func(*testing.T, *MockScanService)
	}{
		{
			name:        "Valid Request - Success",
			requestBody: `{"scan_type":"subdomain_alive","domain":"example.com"}`,
			setupMock: func(m *MockScanService) {
				m.On("StartScan", mock.MatchedBy(func(scan *models.Scan) bool {
					return scan.ScanType == "subdomain_alive" &&
						scan.Domain == "example.com"
				})).Return("123e4567-e89b-12d3-a456-426614174000", nil)
			},
			expectedStatus: 200,
			expectedBody:   `{"scan_id":"123e4567-e89b-12d3-a456-426614174000"}`,
			validateMock: func(t *testing.T, m *MockScanService) {
				m.AssertNumberOfCalls(t, "StartScan", 1)
			},
		},
		{
			name:           "Invalid JSON - Malformed",
			requestBody:    `{"scan_type":"subdomain_alive","domain":}`,
			setupMock:      func(m *MockScanService) {},
			expectedStatus: 400,
			expectedBody:   `{"error":"Invalid request payload"}`,
			validateMock: func(t *testing.T, m *MockScanService) {
				m.AssertNumberOfCalls(t, "StartScan", 0)
			},
		},
		{
			name:           "Missing Required Field - scan_type",
			requestBody:    `{"domain":"example.com"}`,
			setupMock:      func(m *MockScanService) {},
			expectedStatus: 400,
			expectedBody:   `{"error":"Invalid request payload"}`,
		},
		{
			name:           "Missing Required Field - domain",
			requestBody:    `{"scan_type":"subdomain_alive"}`,
			setupMock:      func(m *MockScanService) {},
			expectedStatus: 400,
			expectedBody:   `{"error":"Invalid request payload"}`,
		},
		{
			name:        "Service Error - Internal Error",
			requestBody: `{"scan_type":"subdomain_alive","domain":"example.com"}`,
			setupMock: func(m *MockScanService) {
				m.On("StartScan", mock.AnythingOfType("*models.Scan")).
					Return("", errors.New("database connection failed"))
			},
			expectedStatus: 500,
			expectedBody:   `{"error":"Failed to start scan"}`,
		},
		{
			name:           "Empty Request Body",
			requestBody:    `{}`,
			setupMock:      func(m *MockScanService) {},
			expectedStatus: 400,
			expectedBody:   `{"error":"Invalid request payload"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockScanService)

			tt.setupMock(mockService)

			handler := NewScanHandler(mockService)

			router := gin.New() // Use gin.New() instead of Default() to avoid middleware
			router.POST("/api/scans", handler.StartScan)

			req, err := http.NewRequest("POST", "/api/scans", strings.NewReader(tt.requestBody))
			assert.NoError(t, err, "Failed to create request")
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code,
				"Expected status %d, got %d. Response: %s",
				tt.expectedStatus, w.Code, w.Body.String())

			assert.JSONEq(t, tt.expectedBody, w.Body.String(),
				"Response body doesn't match expected JSON")

			if tt.validateMock != nil {
				tt.validateMock(t, mockService)
			}

			mockService.AssertExpectations(t)
		})
	}
}

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
					Return(nil, services.ErrScanNotFound)
			},
			expectedStatus: 404,
			expectedBody:   `{"error":"Scan not found"}`,
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

func TestDeleteScan(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		scanID         string
		setupMock      func(*MockScanService)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:   "Successful Deletion",
			scanID: "uuid-123",
			setupMock: func(m *MockScanService) {
				m.On("DeleteScan", "uuid-123").Return(nil)
			},
			expectedStatus: 204,
			expectedBody:   "",
		},
		{
			name:   "Scan Not Found",
			scanID: "missing-id",
			setupMock: func(m *MockScanService) {
				m.On("DeleteScan", "missing-id").Return(gorm.ErrRecordNotFound)
			},
			expectedStatus: 404,
			expectedBody:   `{"error":"Scan not found"}`,
		},
		{
			name:   "Service Error",
			scanID: "uuid-987",
			setupMock: func(m *MockScanService) {
				m.On("DeleteScan", "uuid-987").Return(errors.New("db error"))
			},
			expectedStatus: 500,
			expectedBody:   `{"error":"Failed to delete scan"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockScanService)
			tt.setupMock(mockService)

			handler := NewScanHandler(mockService)
			router := gin.New()
			router.DELETE("/api/scans/:id", handler.DeleteScan)

			url := fmt.Sprintf("/api/scans/%s", tt.scanID)
			req, _ := http.NewRequest("DELETE", url, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedBody != "" {
				assert.JSONEq(t, tt.expectedBody, w.Body.String())
			} else {
				assert.Equal(t, "", w.Body.String())
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
