package matcher

import (
	"encoding/base64"
	"strings"
)

func cookieHeaderValues(headers []string, name string) []string {
	values := make([]string, 0)
	for _, header := range headers {
		values = append(values, cookieValues(header, name)...)
	}
	return values
}

func cookieValues(header string, name string) []string {
	parts := strings.Split(header, ";")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && key == name {
			values = append(values, value)
		}
	}
	return values
}

func parseBasicAuth(headers []string) (string, string, bool) {
	for _, header := range headers {
		username, password, ok := decodeBasicAuthHeader(header)
		if ok {
			return username, password, true
		}
	}
	return "", "", false
}

func decodeBasicAuthHeader(header string) (string, string, bool) {
	scheme, encoded, ok := strings.Cut(strings.TrimSpace(header), " ")
	if !ok || !strings.EqualFold(scheme, "Basic") {
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return "", "", false
	}
	username, password, ok := strings.Cut(string(decoded), ":")
	return username, password, ok
}
