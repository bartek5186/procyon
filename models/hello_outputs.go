package models

type HealthResponse struct {
	Status string `json:"status"`
	App    string `json:"app"`
}

type HelloResponse struct {
	App           string `json:"app"`
	Message       string `json:"message"`
	Locale        string `json:"locale"`
	Authenticated bool   `json:"authenticated"`
	IdentityID    string `json:"identity_id,omitempty"`
	Role          string `json:"role,omitempty"`
	Source        string `json:"source"`
}
