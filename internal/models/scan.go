package models

type Subdomain struct {
	Domain              string   `json:"domain"`
	OpenPorts           []string `json:"open_ports,omitempty"`
	PotentialFalsePorts []string `json:"potential_false_ports,omitempty"`
	Vulns               []string `json:"vulns,omitempty"`
	DirFuzzing          []string `json:"dir_fuzzing,omitempty"`
	Screenshot          string   `json:"screenshot,omitempty"`
	Status              string   `json:"status,omitempty"` // alive, dead, etc.
}

type ToolFailure struct {
	ToolName string `json:"tool_name"`
	Error    string `json:"error"`
}

type Scan struct {
	UUID              string        `gorm:"primaryKey;type:varchar(36)" json:"uuid"`
	ScanType          string        `json:"scan_type"`
	Status            string        `json:"status"`
	Domain            string        `json:"domain"`
	NumberOfDomains   int           `json:"number_of_domains"`
	Subdomains        []Subdomain   `gorm:"serializer:json" json:"subdomains"`
	ScreenshotsPath   string        `json:"screenshots_path"`
	SensitivePatterns string        `gorm:"type:text" json:"sensitive_patterns,omitempty"`
	ErrorMessage      string        `gorm:"type:text" json:"error_message,omitempty"`
	FailedTools       []ToolFailure `gorm:"serializer:json" json:"failed_tools,omitempty"`
	CreatedAt         int64         `json:"created_at"`
	UpdatedAt         int64         `json:"updated_at"`
}
