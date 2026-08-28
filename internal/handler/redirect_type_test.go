package handler

import (
	"testing"
)

func TestDetermineRedirectTypeFromString(t *testing.T) {
	tests := []struct {
		name            string
		redirectTypeStr string
		method          string
		defaultType     RedirectType
		expected        RedirectType
	}{
		{
			name:            "301 GET request",
			redirectTypeStr: "301",
			method:          "GET",
			defaultType:     RedirectTypeTemporary,
			expected:        RedirectTypePermanent,
		},
		{
			name:            "301 HEAD request",
			redirectTypeStr: "301",
			method:          "HEAD",
			defaultType:     RedirectTypeTemporary,
			expected:        RedirectTypePermanent,
		},
		{
			name:            "301 POST request",
			redirectTypeStr: "301",
			method:          "POST",
			defaultType:     RedirectTypeTemporary,
			expected:        RedirectTypePermanentPost,
		},
		{
			name:            "301 PUT request",
			redirectTypeStr: "301",
			method:          "PUT",
			defaultType:     RedirectTypeTemporary,
			expected:        RedirectTypePermanentPost,
		},
		{
			name:            "301 PATCH request",
			redirectTypeStr: "301",
			method:          "PATCH",
			defaultType:     RedirectTypeTemporary,
			expected:        RedirectTypePermanentPost,
		},
		{
			name:            "301 DELETE request",
			redirectTypeStr: "301",
			method:          "DELETE",
			defaultType:     RedirectTypeTemporary,
			expected:        RedirectTypePermanentPost,
		},
		{
			name:            "302 GET request",
			redirectTypeStr: "302",
			method:          "GET",
			defaultType:     RedirectTypePermanent,
			expected:        RedirectTypeTemporary,
		},
		{
			name:            "302 HEAD request",
			redirectTypeStr: "302",
			method:          "HEAD",
			defaultType:     RedirectTypePermanent,
			expected:        RedirectTypeTemporary,
		},
		{
			name:            "302 POST request",
			redirectTypeStr: "302",
			method:          "POST",
			defaultType:     RedirectTypePermanent,
			expected:        RedirectTypeTemporaryPost,
		},
		{
			name:            "302 PUT request",
			redirectTypeStr: "302",
			method:          "PUT",
			defaultType:     RedirectTypePermanent,
			expected:        RedirectTypeTemporaryPost,
		},
		{
			name:            "302 PATCH request",
			redirectTypeStr: "302",
			method:          "PATCH",
			defaultType:     RedirectTypePermanent,
			expected:        RedirectTypeTemporaryPost,
		},
		{
			name:            "302 DELETE request",
			redirectTypeStr: "302",
			method:          "DELETE",
			defaultType:     RedirectTypePermanent,
			expected:        RedirectTypeTemporaryPost,
		},
		{
			name:            "Unknown redirect type falls back to default logic",
			redirectTypeStr: "303",
			method:          "GET",
			defaultType:     RedirectTypePermanent,
			expected:        RedirectTypePermanent,
		},
		{
			name:            "Empty redirect type falls back to default logic",
			redirectTypeStr: "",
			method:          "POST",
			defaultType:     RedirectTypePermanent,
			expected:        RedirectTypeTemporaryPost,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineRedirectTypeFromString(tt.redirectTypeStr, tt.method, tt.defaultType)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDetermineRedirectType(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		defaultType RedirectType
		expected    RedirectType
	}{
		{
			name:        "GET request with permanent default",
			method:      "GET",
			defaultType: RedirectTypePermanent,
			expected:    RedirectTypePermanent,
		},
		{
			name:        "HEAD request with permanent default",
			method:      "HEAD",
			defaultType: RedirectTypePermanent,
			expected:    RedirectTypePermanent,
		},
		{
			name:        "POST request with permanent default",
			method:      "POST",
			defaultType: RedirectTypePermanent,
			expected:    RedirectTypeTemporaryPost,
		},
		{
			name:        "PUT request with permanent default",
			method:      "PUT",
			defaultType: RedirectTypePermanent,
			expected:    RedirectTypeTemporaryPost,
		},
		{
			name:        "PATCH request with permanent default",
			method:      "PATCH",
			defaultType: RedirectTypePermanent,
			expected:    RedirectTypeTemporaryPost,
		},
		{
			name:        "DELETE request with permanent default",
			method:      "DELETE",
			defaultType: RedirectTypePermanent,
			expected:    RedirectTypeTemporaryPost,
		},
		{
			name:        "Unknown method with permanent default",
			method:      "UNKNOWN",
			defaultType: RedirectTypePermanent,
			expected:    RedirectTypeTemporary,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineRedirectType(tt.method, tt.defaultType)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
