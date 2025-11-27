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

type PaginationRequest struct {
	Page  int `form:"page" json:"page"`
	Limit int `form:"limit" json:"limit"`
}

type PaginationMeta struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	Total      int  `json:"total"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

type PaginatedScansResponse struct {
	Scans      interface{}    `json:"scans"`
	Pagination PaginationMeta `json:"pagination"`
}
