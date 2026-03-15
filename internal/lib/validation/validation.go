package validation

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ValidateImageURL ensures the URL is a valid public image URL (S3, CDN, etc)
// and rejects blob URLs, localhost, and data URLs.
func ValidateImageURL(urlStr string) error {
	if strings.HasPrefix(urlStr, "blob:") {
		return errors.New("blob URLs are not allowed; upload image via /upload endpoint first and use the returned S3 URL")
	}
	if strings.HasPrefix(urlStr, "data:") {
		return errors.New("data URLs are not allowed; upload image via /upload endpoint first and use the returned S3 URL")
	}

	parsed, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return fmt.Errorf("URLs must use http or https scheme, got: %s", parsed.Scheme)
	}

	// Reject localhost and private IPs
	host := parsed.Hostname()
	if host == "localhost" || host == "127.0.0.1" || strings.HasPrefix(host, "192.168.") || strings.HasPrefix(host, "10.") {
		return fmt.Errorf("localhost and private IP addresses are not allowed; use public S3 or CDN URLs")
	}

	return nil
}
