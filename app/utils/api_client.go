package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// MeUser represents the shape returned by Nextgen /api/me
type MeUser struct {
	ID              string         `json:"id"`
	Email           string         `json:"email,omitempty"`
	FirstName       *string        `json:"firstName"`
	LastName        *string        `json:"lastName"`
	ImageURL        string         `json:"imageUrl"`
	EmailAddresses  []string       `json:"emailAddresses"`
	PublicMetadata  map[string]any `json:"publicMetadata"`
	PrivateMetadata map[string]any `json:"privateMetadata"`
}

type MeResponse struct {
	User MeUser `json:"user"`
}

// GetBaseURL returns the API base URL from env, with a sensible default for dev.
func GetBaseURL() string {
	baseURL := os.Getenv("NEXTGEN_BASE_URL")
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "http://localhost:3000"
	}
	return strings.TrimRight(baseURL, "/")
}

// GetMePath allows overriding the /api/me path via env NEXTGEN_ME_PATH
func GetMePath() string {
	p := os.Getenv("NEXTGEN_ME_PATH")
	if strings.TrimSpace(p) == "" {
		p = "/api/me"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

// FetchMe calls GET {baseURL}/api/me using the given token.
func FetchMe(token string) (MeResponse, error) {
	var out MeResponse
	if strings.TrimSpace(token) == "" {
		return out, errors.New("empty token")
	}
	baseURL := GetBaseURL()
	client := &http.Client{Timeout: 15 * time.Second}

	do := func(url string) (MeResponse, *http.Response, error) {
		var respBody MeResponse
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return respBody, nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err != nil {
			return respBody, nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return respBody, resp, fmt.Errorf("unexpected status %s for %s", resp.Status, url)
		}
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
			return respBody, resp, err
		}
		return respBody, resp, nil
	}

	url := baseURL + GetMePath()
	me, resp, err := do(url)
	if err == nil {
		return me, nil
	}
	// If 404 or 401, try the opposite host as a fallback (helpful when prod/local tokens differ)
	if resp != nil && (resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized) {
		var fallback string
		if strings.Contains(baseURL, "localhost:3000") {
			fallback = "https://nextgen-theme.vercel.app"
		} else {
			fallback = "http://localhost:3000"
		}
		if me2, _, err2 := do(fallback + GetMePath()); err2 == nil {
			return me2, nil
		}
	}
	return out, err
}
