package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// APIError is a non-2xx response from the GW2 API.
type APIError struct {
	StatusCode int
	Body       string
	Hint       string
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("api %d", e.StatusCode)
	if t := extractText(e.Body); t != "" {
		msg += ": " + t
	}
	if e.Hint != "" {
		msg += " (" + e.Hint + ")"
	}
	return msg
}

func apiErrorFrom(code int, body []byte) *APIError {
	e := &APIError{StatusCode: code, Body: string(body)}
	// The GW2 API is inconsistent about auth failures: many authenticated
	// endpoints answer 401 ("Invalid access token") for a missing/revoked
	// key, others 403. Both are authentication errors and get the same
	// hints and exit code.
	if code == http.StatusUnauthorized || code == http.StatusForbidden {
		text := strings.ToLower(extractText(e.Body))
		switch {
		case strings.Contains(text, "invalid key") || strings.Contains(text, "invalid access token"):
			e.Hint = "check your API key (gw2 auth set)"
		case strings.Contains(text, "requires scope") || strings.Contains(text, "permission"):
			e.Hint = "your key lacks a required permission; see gw2 token info"
		default:
			e.Hint = "authentication required or insufficient permission"
		}
	}
	return e
}

func extractText(body string) string {
	var m struct {
		Text  string `json:"text"`
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(body), &m) == nil {
		if m.Text != "" {
			return m.Text
		}
		if m.Error != "" {
			return m.Error
		}
	}
	return ""
}

// ExitCode maps an error to a process exit code.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var ae *APIError
	if errors.As(err, &ae) {
		switch ae.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return 3
		case http.StatusNotFound:
			return 4
		case http.StatusTooManyRequests:
			return 5
		}
	}
	return 1
}
