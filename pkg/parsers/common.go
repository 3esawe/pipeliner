package parsers

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"strings"

	"pipeliner/pkg/logger"

	"github.com/sirupsen/logrus"
)

type CommandParser interface {
	Parse(outputFile string) (map[string]any, error)
}

type NmapParser struct {
	logger *logger.Logger
}

type FuffParser struct {
	logger *logger.Logger
}

func NewNmapParser() *NmapParser {
	return &NmapParser{logger: logger.NewLogger(logrus.InfoLevel)}
}

func NewFuffParser() *FuffParser {
	return &FuffParser{logger: logger.NewLogger(logrus.InfoLevel)}
}

func (p *NmapParser) Parse(outputFile string) (map[string]any, error) {
	if p.logger == nil {
		p.logger = logger.NewLogger(logrus.InfoLevel)
	}
	return p.parseNmapOutput(outputFile)
}

func (p *NmapParser) parseNmapOutput(outputFile string) (map[string]any, error) {
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		p.logger.Errorf("Nmap output file does not exist: %s", outputFile)
		return nil, fmt.Errorf("nmap output file does not exist: %w", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		p.logger.Errorf("Failed to read Nmap output file: %v", err)
		return nil, fmt.Errorf("failed to read nmap output file: %w", err)
	}

	var nmapResult NmapRun
	if err := xml.Unmarshal(data, &nmapResult); err != nil {
		p.logger.Warnf("Nmap XML parsing failed - scan may still be running or incomplete: %v", err)
		return nil, fmt.Errorf("nmap scan not ready or incomplete (XML parse error: %w). This is normal if the scan is still running", err)
	}

	result := make(map[string]any)
	hosts := make([]map[string]any, 0, len(nmapResult.Hosts))

	for _, host := range nmapResult.Hosts {
		isSuspicious := isLikelyFalsePositive(host)

		hostInfo := map[string]any{
			"addresses":             host.Addresses,
			"ports":                 host.Ports.PortList,
			"hostnames":             host.Hostnames.HostnameList,
			"likely_false_positive": isSuspicious,
		}
		hosts = append(hosts, hostInfo)
	}
	result["hosts"] = hosts

	p.logger.Infof("Successfully parsed %d hosts from Nmap output", len(hosts))
	return result, nil
}

func isLikelyFalsePositive(host Host) bool {
	var portCount int
	for _, port := range host.Ports.PortList {
		if port.State.State == "open" {
			portCount++
		}
	}

	return portCount > 20
}

func (p *FuffParser) Parse(outputFile string) (map[string]any, error) {
	if p.logger == nil {
		p.logger = logger.NewLogger(logrus.InfoLevel)
	}
	return p.parseFuffOutput(outputFile)
}

func (p *FuffParser) parseFuffOutput(outputFile string) (map[string]any, error) {
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		p.logger.Errorf("Ffuf output file does not exist: %s", outputFile)
		return nil, fmt.Errorf("ffuf output file does not exist: %w", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		p.logger.Errorf("Failed to read Ffuf output file: %v", err)
		return nil, fmt.Errorf("failed to read ffuf output file: %w", err)
	}

	var fuffResult FuffOutput
	if err := json.Unmarshal(data, &fuffResult); err != nil {
		p.logger.Errorf("Failed to parse Ffuf JSON output: %v", err)
		return nil, fmt.Errorf("failed to parse ffuf JSON output: %w", err)
	}

	result := map[string]any{
		"commandline": fuffResult.Commandline,
		"time":        fuffResult.Time,
		"results":     fuffResult.Results,
	}

	p.logger.Infof("Successfully parsed %d results from Ffuf output", len(fuffResult.Results))
	return result, nil
}

type NucleiParser struct {
	logger *logger.Logger
}

func NewNucleiParser() *NucleiParser {
	return &NucleiParser{logger: logger.NewLogger(logrus.InfoLevel)}
}

func (p *NucleiParser) Parse(outputFile string) (map[string]any, error) {
	if p.logger == nil {
		p.logger = logger.NewLogger(logrus.InfoLevel)
	}
	return p.parseNucleiOutput(outputFile)
}

func (p *NucleiParser) parseNucleiOutput(outputFile string) (map[string]any, error) {
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		p.logger.Errorf("Nuclei output file does not exist: %s", outputFile)
		return nil, fmt.Errorf("nuclei output file does not exist: %w", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		p.logger.Errorf("Failed to read Nuclei output file: %v", err)
		return nil, fmt.Errorf("failed to read nuclei output file: %w", err)
	}

	var results []NucleiResult
	lines := splitLines(data)

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		var result NucleiResult
		if err := json.Unmarshal(line, &result); err != nil {
			p.logger.Warnf("Failed to parse nuclei JSON line: %v", err)
			continue
		}
		results = append(results, result)
	}

	resultMap := map[string]any{
		"results": results,
		"count":   len(results),
	}

	p.logger.Infof("Successfully parsed %d results from Nuclei output", len(results))
	return resultMap, nil
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			if i > start {
				lines = append(lines, data[start:i])
			}
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

func GetNucleiSeverity(info map[string]interface{}) string {
	if severity, ok := info["severity"].(string); ok {
		return strings.ToLower(severity)
	}
	return "info"
}

func GetNucleiTemplateName(info map[string]interface{}) string {
	if name, ok := info["name"].(string); ok {
		return name
	}
	return "Unknown Template"
}

func GetNucleiDescription(info map[string]interface{}) string {
	if desc, ok := info["description"].(string); ok {
		return strings.TrimSpace(desc)
	}
	return ""
}

func GetNucleiTags(info map[string]interface{}) string {
	if tags, ok := info["tags"].([]interface{}); ok {
		var tagStrs []string
		for _, t := range tags {
			if s, ok := t.(string); ok {
				tagStrs = append(tagStrs, s)
			}
		}
		if len(tagStrs) > 5 {
			tagStrs = tagStrs[:5]
		}
		return strings.Join(tagStrs, ", ")
	}
	return ""
}
