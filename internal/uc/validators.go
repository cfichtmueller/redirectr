package uc

import (
	"regexp"
	"strings"

	"github.com/cfichtmueller/srv"
)

var (
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	// Domain regex: allows subdomains, hyphens, and standard TLDs
	// Matches: example.com, sub.example.com, example-site.co.uk, etc.
	domainRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?))*\.[a-zA-Z]{2,}$`)
)

func RequireEmail(field string, value string, prev *srv.ValidationError) *srv.ValidationError {
	v := srv.RequireNotEmpty(field, value, prev)
	if value == "" {
		return v
	}

	return srv.RequireRegex(field, value, emailRegex, v)
}

func RequireUserPassword(field string, value string, prev *srv.ValidationError) *srv.ValidationError {
	v := srv.RequireNotEmpty(field, value, prev)
	v = srv.RequireMinLength(field, 8, value, v)
	v = srv.RequireMaxLength(field, 63, value, v)
	return v
}

// RequireDomain validates that a domain string is properly formatted
func RequireDomain(field string, value string, prev *srv.ValidationError) *srv.ValidationError {
	v := srv.RequireNotEmpty(field, value, prev)
	if value == "" {
		return v
	}

	// Normalize domain to lowercase for validation
	normalizedValue := strings.ToLower(strings.TrimSpace(value))

	// Check basic length constraints
	v = srv.RequireMaxLength(field, 253, normalizedValue, v)
	v = srv.RequireMinLength(field, 4, normalizedValue, v)

	// Check domain format with regex
	return srv.RequireRegex(field, normalizedValue, domainRegex, v)
}
