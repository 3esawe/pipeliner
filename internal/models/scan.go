package models

type Subdomain struct {
	Domain     string   `json:"domain"`
	OpenPorts  []string `json:"open_ports,omitempty"`
	Vulns      []string `json:"vulns,omitempty"`
	DirFuzzing []string `json:"dir_fuzzing,omitempty"`
	Screenshot string   `json:"screenshot,omitempty"`
	Status     string   `json:"status,omitempty"` // alive, dead, etc.
}

type Scan struct {
	UUID            string      `gorm:"primaryKey;type:varchar(36)" json:"uuid"`
	ScanType        string      `json:"scan_type"`
	Status          string      `json:"status"`
	Domain          string      `json:"domain"`
	NumberOfDomains int         `json:"number_of_domains"`
	Subdomains      []Subdomain `gorm:"serializer:json" json:"subdomains"`
	ScreenshotsPath string      `json:"screenshots_path"`
	CreatedAt       int64       `json:"created_at"`
	UpdatedAt       int64       `json:"updated_at"`
}
