package redirect

import (
	"testing"
)

// Redirect Type Tests

func TestRedirectTypes(t *testing.T) {
	expectedTypes := []string{"301", "302"}

	if len(RedirectTypes) != len(expectedTypes) {
		t.Errorf("Expected %d redirect types, got %d", len(expectedTypes), len(RedirectTypes))
	}

	for _, expectedType := range expectedTypes {
		found := false
		for _, actualType := range RedirectTypes {
			if actualType == expectedType {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected redirect type %s not found in RedirectTypes", expectedType)
		}
	}
}

func TestRedirectTypeConstants(t *testing.T) {
	if RedirectType301 != "301" {
		t.Errorf("Expected RedirectType301 to be '301', got '%s'", RedirectType301)
	}

	if RedirectType302 != "302" {
		t.Errorf("Expected RedirectType302 to be '302', got '%s'", RedirectType302)
	}
}

func TestRedirect_RedirectTypeField(t *testing.T) {
	tests := []struct {
		name         string
		redirectType string
		expected     string
	}{
		{
			name:         "301 redirect type",
			redirectType: RedirectType301,
			expected:     "301",
		},
		{
			name:         "302 redirect type",
			redirectType: RedirectType302,
			expected:     "302",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redirect := Redirect{
				SourceDomain: "example.com",
				TargetDomain: "target.com",
				Status:       RedirectStatusActive,
				RedirectType: tt.redirectType,
			}

			if redirect.RedirectType != tt.expected {
				t.Errorf("Expected redirect type %s, got %s", tt.expected, redirect.RedirectType)
			}
		})
	}
}
