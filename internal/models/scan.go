package models

type Scan struct {
	UUID            string `gorm:"primaryKey;type:varchar(36)" json:"uuid"`
	ScanType        string `json:"scan_type"`
	Status          string `json:"status"`
	Domain          string `json:"domain"`
	NumberOfDomains int    `json:"number_of_domains"`
	ScreenshotsPath string `json:"screenshots_path"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
}
