package parsers

type NmapRun struct {
	Hosts []Host `xml:"host" json:"hosts"`
}

type Host struct {
	Addresses []Address `xml:"address" json:"addresses"`
	Ports     Ports     `xml:"ports" json:"ports"`
	Hostnames Hostnames `xml:"hostnames" json:"hostnames"`
}

type Address struct {
	Addr     string `xml:"addr,attr" json:"addr"`
	AddrType string `xml:"addrtype,attr" json:"addrtype"`
}

type Ports struct {
	PortList []Port `xml:"port" json:"port_list"`
}

type Port struct {
	Protocol string  `xml:"protocol,attr" json:"protocol"`
	PortID   string  `xml:"portid,attr" json:"port_id"`
	State    State   `xml:"state" json:"state"`
	Service  Service `xml:"service" json:"service"`
}

type State struct {
	State     string `xml:"state,attr" json:"state"`
	Reason    string `xml:"reason,attr" json:"reason"`
	ReasonTTL string `xml:"reason_ttl,attr" json:"reason_ttl"`
}

type Service struct {
	Name   string `xml:"name,attr" json:"name"`
	Method string `xml:"method,attr" json:"method"`
	Conf   string `xml:"conf,attr" json:"conf"`
}

type Hostnames struct {
	HostnameList []Hostname `xml:"hostname" json:"hostname_list"`
}

type Hostname struct {
	Name string `xml:"name,attr" json:"name"`
	Type string `xml:"type,attr" json:"type"`
}

type FuffOutput struct {
	Commandline string       `json:"commandline"`
	Time        string       `json:"time"`
	Results     []FuffResult `json:"results"`
}

type FuffResult struct {
	Input            map[string]string `json:"input"`
	Position         int               `json:"position"`
	Status           int               `json:"status"`
	Length           int               `json:"length"`
	Words            int               `json:"words"`
	Lines            int               `json:"lines"`
	ContentType      string            `json:"content-type"`
	RedirectLocation string            `json:"redirectlocation"`
	Scraper          map[string]any    `json:"scraper"`
	Duration         int64             `json:"duration"`
	ResultFile       string            `json:"resultfile"`
	URL              string            `json:"url"` // Changed from Url to URL (Go convention)
	Host             string            `json:"host"`
}

type NucleiResult struct {
	TemplateID    string                 `json:"template-id"`
	TemplatePath  string                 `json:"template-path"`
	Info          map[string]interface{} `json:"info"`
	MatcherName   string                 `json:"matcher-name"`
	Type          string                 `json:"type"`
	Host          string                 `json:"host"`
	Port          string                 `json:"port"`
	Scheme        string                 `json:"scheme"`
	URL           string                 `json:"url"`
	MatchedAt     string                 `json:"matched-at"`
	Request       string                 `json:"request"`
	Response      string                 `json:"response"`
	IP            string                 `json:"ip"`
	Timestamp     string                 `json:"timestamp"`
	CurlCommand   string                 `json:"curl-command"`
	MatcherStatus bool                   `json:"matcher-status"`
}
