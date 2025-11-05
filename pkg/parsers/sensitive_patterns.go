package parsers

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

type SensitivePattern struct {
	Pattern     string
	Regex       *regexp.Regexp
	Severity    string
	Description string
	Category    string
}

var defaultPatterns = []SensitivePattern{
	{Pattern: "/actuator", Severity: "critical", Description: "Spring Boot Actuator", Category: "Configuration"},
	{Pattern: "/actuator/env", Severity: "critical", Description: "Spring Boot Environment Exposure", Category: "Configuration"},
	{Pattern: "/actuator/heapdump", Severity: "critical", Description: "Spring Boot Heap Dump", Category: "Configuration"},
	{Pattern: "/actuator/mappings", Severity: "critical", Description: "Spring Boot Route Mappings", Category: "Configuration"},
	{Pattern: "/actuator/trace", Severity: "critical", Description: "Spring Boot HTTP Trace", Category: "Configuration"},
	{Pattern: "/actuator/beans", Severity: "critical", Description: "Spring Boot Beans Configuration", Category: "Configuration"},
	{Pattern: "/.env", Severity: "critical", Description: "Environment Configuration File", Category: "Configuration"},
	{Pattern: "/config.json", Severity: "high", Description: "JSON Configuration File", Category: "Configuration"},
	{Pattern: "/config.yml", Severity: "high", Description: "YAML Configuration File", Category: "Configuration"},
	{Pattern: "/config.yaml", Severity: "high", Description: "YAML Configuration File", Category: "Configuration"},
	{Pattern: "/web.config", Severity: "critical", Description: "IIS Web Configuration", Category: "Configuration"},
	{Pattern: "/application.properties", Severity: "high", Description: "Application Properties File", Category: "Configuration"},
	{Pattern: "/.git", Severity: "critical", Description: "Git Repository Exposed", Category: "Source Code"},
	{Pattern: "/.git/config", Severity: "critical", Description: "Git Configuration", Category: "Source Code"},
	{Pattern: "/.svn", Severity: "critical", Description: "SVN Repository Exposed", Category: "Source Code"},
	{Pattern: "/.aws/credentials", Severity: "critical", Description: "AWS Credentials File", Category: "Credentials"},
	{Pattern: "/credentials", Severity: "critical", Description: "Credentials File", Category: "Credentials"},
	{Pattern: "/.ssh", Severity: "critical", Description: "SSH Keys Directory", Category: "Credentials"},
	{Pattern: ".sql", Severity: "critical", Description: "SQL Database Dump", Category: "Database"},
	{Pattern: "/database.sql", Severity: "critical", Description: "Database Dump File", Category: "Database"},
	{Pattern: "/backup.sql", Severity: "critical", Description: "Database Backup", Category: "Database"},
	{Pattern: "/admin", Severity: "high", Description: "Admin Panel", Category: "Admin"},
	{Pattern: "/administrator", Severity: "high", Description: "Administrator Panel", Category: "Admin"},
	{Pattern: "/console", Severity: "critical", Description: "Web Console", Category: "Admin"},
	{Pattern: "/phpmyadmin", Severity: "high", Description: "phpMyAdmin", Category: "Admin"},
	{Pattern: "/swagger", Severity: "medium", Description: "Swagger API Documentation", Category: "API"},
	{Pattern: "/api-docs", Severity: "medium", Description: "API Documentation", Category: "API"},
	{Pattern: "/graphql", Severity: "medium", Description: "GraphQL Endpoint", Category: "API"},
	{Pattern: "/v2/api-docs", Severity: "medium", Description: "Swagger V2 API Docs", Category: "API"},
	{Pattern: "/phpinfo.php", Severity: "critical", Description: "PHP Info Page", Category: "Information Disclosure"},
	{Pattern: "/server-status", Severity: "high", Description: "Apache Server Status", Category: "Information Disclosure"},
	{Pattern: "/server-info", Severity: "high", Description: "Apache Server Info", Category: "Information Disclosure"},
	{Pattern: "/debug", Severity: "high", Description: "Debug Endpoint", Category: "Debug"},
	{Pattern: "/trace", Severity: "high", Description: "Trace Endpoint", Category: "Debug"},
	{Pattern: "/_debug", Severity: "high", Description: "Debug Endpoint", Category: "Debug"},
	{Pattern: "/backup", Severity: "high", Description: "Backup Directory", Category: "Backup"},
	{Pattern: ".bak", Severity: "high", Description: "Backup File", Category: "Backup"},
	{Pattern: ".old", Severity: "medium", Description: "Old/Backup File", Category: "Backup"},
	{Pattern: ".zip", Severity: "medium", Description: "Archive File", Category: "Backup"},
	{Pattern: "/backup.zip", Severity: "high", Description: "Backup Archive", Category: "Backup"},
}

func LoadSensitivePatternsFromFile(filePath string) ([]SensitivePattern, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var patterns []SensitivePattern
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		re, err := regexp.Compile(line)
		if err != nil {
			continue
		}

		patterns = append(patterns, SensitivePattern{
			Pattern:     line,
			Regex:       re,
			Severity:    "high",
			Description: "Custom Pattern Match",
			Category:    "Custom",
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return patterns, nil
}

func GetDefaultPatterns() []SensitivePattern {
	patterns := make([]SensitivePattern, len(defaultPatterns))
	copy(patterns, defaultPatterns)

	for i := range patterns {
		patterns[i].Regex = regexp.MustCompile(regexp.QuoteMeta(patterns[i].Pattern))
	}

	return patterns
}

func DetectSensitivePattern(url string, customPatternsFile string) (SensitivePattern, bool) {
	urlLower := strings.ToLower(url)

	var patterns []SensitivePattern

	if customPatternsFile != "" {
		if userPatterns, err := LoadSensitivePatternsFromFile(customPatternsFile); err == nil && len(userPatterns) > 0 {
			patterns = userPatterns
		} else {
			patterns = GetDefaultPatterns()
		}
	} else {
		patterns = GetDefaultPatterns()
	}

	for _, pattern := range patterns {
		if pattern.Regex != nil && pattern.Regex.MatchString(urlLower) {
			return pattern, true
		} else if strings.Contains(urlLower, strings.ToLower(pattern.Pattern)) {
			return pattern, true
		}
	}

	return SensitivePattern{}, false
}

func GetSeverityEmoji(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "🔴"
	case "high":
		return "🟠"
	case "medium":
		return "🟡"
	case "low":
		return "🟢"
	case "info":
		return "🔵"
	default:
		return "⚪"
	}
}
