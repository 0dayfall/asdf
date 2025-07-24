package types

// JRD represents a JSON Resource Descriptor
type JRD struct {
	Subject    string                 `json:"subject,omitempty"`
	Aliases    []string               `json:"aliases,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Links      []Link                 `json:"links,omitempty"`
}

// Link represents a link in the JRD
type Link struct {
	Rel  string `json:"rel,omitempty"`
	Type string `json:"type,omitempty"`
	Href string `json:"href,omitempty"`
}
