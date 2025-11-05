package handlers

import "pipeliner/pkg/tools"

type ScanRequest struct {
	ScanType          string `json:"scan_type" binding:"required"`
	Domain            string `json:"domain" binding:"required"`
	SensitivePatterns string `json:"sensitive_patterns"`
}

type ScanResponse struct {
	ScanID string `json:"scan_id" `
}

type ConfigsRequest struct {
}

type ConfigsResponse struct {
	tools.ToolConfig
}
