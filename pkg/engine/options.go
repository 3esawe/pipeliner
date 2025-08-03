package engine

import (
	"os"

	"github.com/jessevdk/go-flags"
	log "github.com/sirupsen/logrus"
)

const (
	ScanSubdomain = "subdomain"
	ScanPort      = "port"
	ScanVuln      = "vuln"
)

type Options struct {
	ScanType string `short:"s" long:"scan" description:"Scan type (subdomain|port|vuln)" required:"true" choice:"subdomain" choice:"port" choice:"vuln"`

	// Subdomain scan options
	Domain string `short:"d" long:"domain" description:"Domain name (required for subdomain scan)" group:"Subdomain Options"`
	Output string `short:"o" long:"output" description:"file output" group:"Subdomain Options"`
	Input  string `short:"i" long:"input" description:"file input for httpx alive" group:"Subdomain Options"`
}

func ParseOptions() *Options {
	options := &Options{}
	flags.Parse(options)
	if options.ScanType == "" {
		log.Error("Scan type is required. Use --scan to specify it.")
		os.Exit(1)
	}

	switch options.ScanType {
	case ScanSubdomain:
		if options.Domain == "" {
			log.Error("Domain is required for subdomain scan. Use --domain")
			os.Exit(1)
		}
		// Add more validation for other scan types as needed
	}
	return options
}
