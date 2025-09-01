package config

// Config represents user settings stored on disk.
// Add more fields if needed (username, expiration, etc.).
type Config struct {
	IsLoggedIn bool   `json:"is_logged_in"`
	Token      string `json:"token,omitempty"`
	UserID     string `json:"user_id,omitempty"`
	Unverified bool   `json:"unverified,omitempty"`
}
