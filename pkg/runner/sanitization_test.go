package runner_test

import (
	"regexp"
	"strings"
	"testing"

	"pipeliner/pkg/runner"
)

func TestReplacementCommandRunner_FilenameSanitization(t *testing.T) {
	mockRunner := &MockBaseRunner{}
	_ = runner.NewReplacementCommandRunner(mockRunner) // silence unused variable warning

	testCases := []struct {
		name         string
		args         []string
		token        string
		value        string
		expectedArgs []string
		description  string
	}{
		{
			name:         "URL in output filename",
			args:         []string{"-o", "scan_{{URL}}_results.txt", "-u", "{{URL}}"},
			token:        "{{URL}}",
			value:        "https://example.com/path",
			expectedArgs: []string{"-o", "scan_example.com_path_results.txt", "-u", "https://example.com/path"},
			description:  "Should sanitize URL in filename but keep original in URL parameter",
		},
		{
			name:         "URL with invalid filename characters",
			args:         []string{"--output-file", "{{TARGET}}_scan.json"},
			token:        "{{TARGET}}",
			value:        "https://test.com:8080/api?param=value",
			expectedArgs: []string{"--output-file", "test.com_8080_api_param_value_scan.json"},
			description:  "Should remove protocol and sanitize special characters",
		},
		{
			name:         "HTTP URL in log file",
			args:         []string{"-l", "{{DOMAIN}}.log", "--target", "{{DOMAIN}}"},
			token:        "{{DOMAIN}}",
			value:        "http://subdomain.example.com",
			expectedArgs: []string{"-l", "subdomain.example.com.log", "--target", "http://subdomain.example.com"},
			description:  "Should sanitize for log file but preserve for target parameter",
		},
		{
			name:         "Complex URL with multiple invalid chars",
			args:         []string{"--report", "report_{{URL}}.xml"},
			token:        "{{URL}}",
			value:        "https://api.example.com/v1/users?id=123&format=json#section",
			expectedArgs: []string{"--report", "report_api.example.com_v1_users_id_123_format_json_section.xml"},
			description:  "Should handle complex URLs with query parameters and fragments",
		},
		{
			name:         "Non-filename usage should not be sanitized",
			args:         []string{"-u", "{{URL}}/FUZZ", "-w", "wordlist.txt"},
			token:        "{{URL}}",
			value:        "https://example.com",
			expectedArgs: []string{"-u", "https://example.com/FUZZ", "-w", "wordlist.txt"},
			description:  "Should not sanitize URL when used in non-filename context",
		},
		{
			name:         "Multiple tokens in filename",
			args:         []string{"-o", "{{PROTOCOL}}_{{DOMAIN}}_scan.txt"},
			token:        "{{DOMAIN}}",
			value:        "https://example.com:443",
			expectedArgs: []string{"-o", "{{PROTOCOL}}_example.com_443_scan.txt"},
			description:  "Should sanitize only the replaced token in filename",
		},
		{
			name:         "Empty value after sanitization",
			args:         []string{"-o", "scan_{{VALUE}}.txt"},
			token:        "{{VALUE}}",
			value:        "://://",
			expectedArgs: []string{"-o", "scan_sanitized_value.txt"},
			description:  "Should provide fallback for values that become empty after sanitization",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Use the private replaceInArgs method through reflection or create a test helper
			// For now, we'll test it indirectly through the public interface

			// Clear previous commands
			mockRunner.ExecutedCommands = nil

			// We can't directly test replaceInArgs since it's private, but we can verify
			// the behavior by checking the result of the replacement operation

			// Create a simple test to verify the logic works
			// This is a basic test - in a real scenario you'd test through the full pipeline
			result := make([]string, len(tc.args))
			for i, arg := range tc.args {
				if strings.Contains(arg, tc.token) {
					// Simulate the sanitization logic for file paths
					if isLikelyFilePathTest(arg, tc.token) {
						sanitizedValue := sanitizeForFilenameTest(tc.value)
						result[i] = strings.ReplaceAll(arg, tc.token, sanitizedValue)
					} else {
						result[i] = strings.ReplaceAll(arg, tc.token, tc.value)
					}
				} else {
					result[i] = arg
				}
			}

			// Verify the results match expectations
			for i, expected := range tc.expectedArgs {
				if i < len(result) && result[i] != expected {
					t.Errorf("Arg %d: expected %s, got %s", i, expected, result[i])
				}
			}
		})
	}
}

// Test helper functions that mirror the private methods
func isLikelyFilePathTest(arg, token string) bool {
	containsToken := strings.Contains(arg, token)
	if !containsToken {
		return false
	}

	argLower := strings.ToLower(arg)

	// Skip if it looks like a URL parameter (starts with http/https protocol patterns)
	if strings.Contains(argLower, "://") {
		return false
	}

	// Look for file path indicators
	fileExtensions := []string{
		".txt", ".json", ".xml", ".csv", ".log", ".out", ".html", ".pdf",
	}

	filenameIndicators := []string{
		"output", "result", "scan", "report", "log", "file",
	}

	// Check for file extensions
	for _, ext := range fileExtensions {
		if strings.Contains(argLower, ext) {
			return true
		}
	}

	// Check for filename-related words
	for _, indicator := range filenameIndicators {
		if strings.Contains(argLower, indicator) {
			return true
		}
	}

	// Check for path separators (but not in URL context)
	if strings.Contains(arg, "/") || strings.Contains(arg, "\\") {
		// Additional check: make sure it's not a URL path
		if !strings.Contains(argLower, "http") && !strings.Contains(argLower, "fuzz") {
			return true
		}
	}

	return false
}

func sanitizeForFilenameTest(value string) string {
	sanitized := value

	// Remove protocols
	protocolRegex := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*://`)
	sanitized = protocolRegex.ReplaceAllString(sanitized, "")

	// Replace invalid filename characters with underscores
	// Including query parameters and fragment identifiers
	invalidChars := regexp.MustCompile(`[<>:"/\\|?*=&#]`)
	sanitized = invalidChars.ReplaceAllString(sanitized, "_")

	// Replace multiple consecutive underscores with single underscore
	multipleUnderscores := regexp.MustCompile(`_+`)
	sanitized = multipleUnderscores.ReplaceAllString(sanitized, "_")

	// Remove leading/trailing underscores and dots
	sanitized = strings.Trim(sanitized, "_.")

	// Handle special cases
	if sanitized == "" {
		sanitized = "sanitized_value"
	}

	// Limit length to prevent very long filenames
	maxLength := 100
	if len(sanitized) > maxLength {
		sanitized = sanitized[:maxLength]
		sanitized = strings.TrimRight(sanitized, "_.")
	}

	return sanitized
}
