package parsers

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
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
		p.logger.Errorf("Failed to parse Nmap XML output: %v", err)
		return nil, fmt.Errorf("failed to parse nmap XML output: %w", err)
	}

	result := make(map[string]any)
	hosts := make([]map[string]any, 0, len(nmapResult.Hosts))

	for _, host := range nmapResult.Hosts {
		hostInfo := map[string]any{
			"addresses": host.Addresses,
			"ports":     host.Ports.PortList,
			"hostnames": host.Hostnames.HostnameList,
		}
		hosts = append(hosts, hostInfo)
	}
	result["hosts"] = hosts

	p.logger.Infof("Successfully parsed %d hosts from Nmap output", len(hosts))
	return result, nil
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
