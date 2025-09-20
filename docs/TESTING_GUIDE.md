# Unit Testing Guide for Pipeliner Handlers

This guide explains the comprehensive unit testing approach implemented for the Pipeliner scan handlers.

## Overview

The test suite demonstrates modern Go testing best practices including:
- **Mocking dependencies** with testify/mock
- **Table-driven testing** for comprehensive coverage
- **HTTP testing** with httptest package
- **Assertion-based validation** for clear test outcomes
- **Performance benchmarking** to measure handler performance

## Test Architecture

### Mock Service Layer
```go
type MockScanService struct {
    mock.Mock // Embeds testify/mock functionality
}
```

**Key Benefits:**
- **Isolation**: Tests run independently of real database/services
- **Control**: Precisely control mock behavior for different scenarios
- **Speed**: Fast execution without external dependencies
- **Deterministic**: Consistent results every time

### Table-Driven Testing Pattern
```go
tests := []struct {
    name           string
    requestBody    string
    setupMock      func(*MockScanService)
    expectedStatus int
    expectedBody   string
}{
    // Test cases...
}
```

**Advantages:**
- **Comprehensive Coverage**: Test multiple scenarios with same logic
- **Maintainable**: Easy to add new test cases
- **Clear Structure**: Each test case is self-documenting
- **DRY Principle**: Avoid duplicated test code

## Test Categories Covered

### 1. Happy Path Testing
```go
{
    name: "Valid Request - Success",
    requestBody: `{"scan_type":"subdomain_alive","domain":"example.com"}`,
    setupMock: func(m *MockScanService) {
        m.On("StartScan", mock.MatchedBy(func(scan *models.Scan) bool {
            return scan.ScanType == "subdomain_alive" && scan.Domain == "example.com"
        })).Return("123e4567-e89b-12d3-a456-426614174000", nil)
    },
    expectedStatus: 200,
    expectedBody: `{"scan_id":"123e4567-e89b-12d3-a456-426614174000"}`,
}
```

### 2. Input Validation Testing
```go
{
    name: "Invalid JSON - Malformed",
    requestBody: `{"scan_type":"subdomain_alive","domain":}`, // Missing value
    expectedStatus: 400,
    expectedBody: `{"error":"Invalid request payload"}`,
}
```

### 3. Error Handling Testing
```go
{
    name: "Service Error - Internal Error",
    setupMock: func(m *MockScanService) {
        m.On("StartScan", mock.AnythingOfType("*models.Scan")).
          Return("", errors.New("database connection failed"))
    },
    expectedStatus: 500,
    expectedBody: `{"error":"Failed to start scan"}`,
}
```

### 4. Edge Case Testing
- Empty request bodies
- Missing required fields
- Service returning nil values
- Network/timeout errors

## Key Testing Techniques Explained

### 1. Mock Setup with Argument Matching
```go
m.On("StartScan", mock.MatchedBy(func(scan *models.Scan) bool {
    return scan.ScanType == "subdomain_alive" && scan.Domain == "example.com"
})).Return("test-id", nil)
```

**Purpose**: Verify the handler passes correct data to the service layer.

### 2. HTTP Request/Response Testing
```go
req, err := http.NewRequest("POST", "/api/scans", strings.NewReader(requestBody))
w := httptest.NewRecorder()
router.ServeHTTP(w, req)
```

**Benefits**: Tests the complete HTTP flow including routing, middleware, and response generation.

### 3. JSON Response Validation
```go
assert.JSONEq(t, expectedBody, w.Body.String())
```

**Advantage**: Compares JSON semantically, ignoring whitespace differences.

### 4. Mock Call Verification
```go
mockService.AssertNumberOfCalls(t, "StartScan", 1)
mockService.AssertExpectations(t)
```

**Purpose**: Ensures mocked methods are called the expected number of times with correct arguments.

## Performance Testing

### Benchmark Test
```go
func BenchmarkStartScan(b *testing.B) {
    // Setup...
    b.ResetTimer() // Don't count setup time
    for i := 0; i < b.N; i++ {
        // Execute handler
    }
}
```

**Results Interpretation:**
- `18541 ns/op`: Average time per operation (18.5 microseconds)
- `9425 B/op`: Memory allocated per operation  
- `115 allocs/op`: Number of memory allocations per operation

## Running the Tests

### Unit Tests
```bash
# Run all handler tests with verbose output
go test ./internal/handlers -v

# Run specific test
go test ./internal/handlers -run TestStartScan -v

# Run with coverage
go test ./internal/handlers -cover
```

### Benchmark Tests
```bash
# Run benchmarks
go test ./internal/handlers -bench=. -benchmem

# Run only benchmarks
go test ./internal/handlers -run=^$ -bench=.
```

## Test Data Management

### Helper Functions
```go
func createScanRequestBody(scanType, domain string) string {
    req := ScanRequest{ScanType: scanType, Domain: domain}
    body, _ := json.Marshal(req)
    return string(body)
}
```

**Benefits**: 
- Consistent test data generation
- Type-safe request creation
- Reduces test maintenance overhead

## Best Practices Demonstrated

### 1. Test Isolation
- Each test creates its own mock instance
- No shared state between tests
- Independent test execution

### 2. Clear Test Names
- Descriptive test case names
- Self-documenting test purposes
- Easy identification of failing scenarios

### 3. Comprehensive Assertions
- Status code validation
- Response body verification
- Mock interaction confirmation

### 4. Error Path Coverage
- Invalid input handling
- Service layer failures
- Edge case scenarios

## Integration with CI/CD

### Test Commands for Automation
```bash
# Full test suite with coverage
go test ./... -cover -race

# Generate coverage report
go test ./internal/handlers -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

### Test Metrics to Track
- **Code Coverage**: Percentage of code exercised by tests
- **Test Execution Time**: Monitor for performance regressions
- **Test Reliability**: Ensure consistent pass rates
- **Mock Coverage**: Verify all service interactions are tested

## Extending the Test Suite

### Adding New Test Cases
1. **Identify the scenario** to test
2. **Create test data** for the scenario
3. **Setup mock behavior** if needed
4. **Define expected outcomes**
5. **Add to the test table**

### Testing New Handlers
1. **Create mock implementations** for dependencies
2. **Follow the same table-driven pattern**
3. **Include happy path, error cases, and edge cases**
4. **Add benchmark tests** for performance validation

This testing approach ensures robust, maintainable, and reliable handler code that can confidently handle production workloads.
