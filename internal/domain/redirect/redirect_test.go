package redirect

import (
	"context"
	"testing"

	"github.com/cfichtmueller/redirectr/internal/ec"
)

func TestCheckForCircularRedirect_DirectOnly(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		sourceDomain  string
		targetDomain  string
		expectedError error
	}{
		{
			name:          "Direct circular redirect",
			sourceDomain:  "example.com",
			targetDomain:  "example.com",
			expectedError: ec.CircularRedirectNotAllowed,
		},
		{
			name:          "Case insensitive direct circular redirect",
			sourceDomain:  "EXAMPLE.COM",
			targetDomain:  "example.com",
			expectedError: ec.CircularRedirectNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckForCircularRedirect(ctx, tt.sourceDomain, tt.targetDomain)
			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Expected error %v, got nil", tt.expectedError)
				} else if err != tt.expectedError {
					t.Errorf("Expected error %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

// TestCheckForCircularRedirect_Indirect tests indirect circular redirect detection
// Note: This test requires a proper database setup and would need to be run
// in an integration test environment with a test database
func TestCheckForCircularRedirect_Indirect(t *testing.T) {
	t.Skip("Skipping indirect circular redirect test - requires database setup")

	// This test would need:
	// 1. A test database connection
	// 2. Setup of existing redirects
	// 3. Testing of indirect cycles like A->B->A
	//
	// Example test cases that would be implemented:
	// - A->B->A cycle detection
	// - A->B->C->A cycle detection
	// - Complex cycles like A->B->C->B
	// - Long chains without cycles
	// - Inactive/deleted redirects being ignored
}

func TestRedirect_IsCircular(t *testing.T) {
	tests := []struct {
		name     string
		redirect Redirect
		expected bool
	}{
		{
			name: "Direct circular redirect",
			redirect: Redirect{
				SourceDomain: "example.com",
				TargetDomain: "example.com",
			},
			expected: true,
		},
		{
			name: "Case insensitive circular redirect",
			redirect: Redirect{
				SourceDomain: "EXAMPLE.COM",
				TargetDomain: "example.com",
			},
			expected: true,
		},
		{
			name: "Not circular",
			redirect: Redirect{
				SourceDomain: "example.com",
				TargetDomain: "target.com",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.redirect.IsCircular()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// Benchmark tests for performance
func BenchmarkCheckForCircularRedirect_DirectCircular(b *testing.B) {
	ctx := context.Background()
	sourceDomain := "example.com"
	targetDomain := "example.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CheckForCircularRedirect(ctx, sourceDomain, targetDomain)
	}
}

// Note: BenchmarkCheckForCircularRedirect_Simple is skipped because it would require
// database access to test indirect circular redirect detection
func BenchmarkCheckForCircularRedirect_Simple(b *testing.B) {
	b.Skip("Skipping simple benchmark - requires database setup for indirect cycle detection")
}

// UTM Tags Tests

func TestUTMTags_HasUTMTags(t *testing.T) {
	tests := []struct {
		name     string
		utmTags  *UTMTags
		expected bool
	}{
		{
			name:     "Nil UTM tags",
			utmTags:  nil,
			expected: false,
		},
		{
			name: "Empty UTM tags",
			utmTags: &UTMTags{
				Source:   "",
				Medium:   "",
				Campaign: "",
				Term:     "",
				Content:  "",
			},
			expected: false,
		},
		{
			name: "Has source",
			utmTags: &UTMTags{
				Source: "google",
			},
			expected: true,
		},
		{
			name: "Has medium",
			utmTags: &UTMTags{
				Medium: "cpc",
			},
			expected: true,
		},
		{
			name: "Has campaign",
			utmTags: &UTMTags{
				Campaign: "summer-sale",
			},
			expected: true,
		},
		{
			name: "Has term",
			utmTags: &UTMTags{
				Term: "running shoes",
			},
			expected: true,
		},
		{
			name: "Has content",
			utmTags: &UTMTags{
				Content: "banner-ad",
			},
			expected: true,
		},
		{
			name: "Has multiple tags",
			utmTags: &UTMTags{
				Source:   "google",
				Medium:   "cpc",
				Campaign: "summer-sale",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.utmTags.HasUTMTags()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestUTMTags_Validate(t *testing.T) {
	tests := []struct {
		name        string
		utmTags     *UTMTags
		expectError bool
	}{
		{
			name:        "Nil UTM tags",
			utmTags:     nil,
			expectError: false,
		},
		{
			name: "Valid empty UTM tags",
			utmTags: &UTMTags{
				Source:   "",
				Medium:   "",
				Campaign: "",
				Term:     "",
				Content:  "",
			},
			expectError: false,
		},
		{
			name: "Valid UTM tags",
			utmTags: &UTMTags{
				Source:   "google",
				Medium:   "cpc",
				Campaign: "summer-sale",
				Term:     "running shoes",
				Content:  "banner-ad",
			},
			expectError: false,
		},
		{
			name: "Source too long",
			utmTags: &UTMTags{
				Source: "this-is-a-very-long-source-name-that-exceeds-the-maximum-allowed-length-of-one-hundred-characters-and-should-cause-a-validation-error",
			},
			expectError: true,
		},
		{
			name: "Medium too long",
			utmTags: &UTMTags{
				Medium: "this-is-a-very-long-medium-name-that-exceeds-the-maximum-allowed-length-of-one-hundred-characters-and-should-cause-a-validation-error",
			},
			expectError: true,
		},
		{
			name: "Campaign too long",
			utmTags: &UTMTags{
				Campaign: "this-is-a-very-long-campaign-name-that-exceeds-the-maximum-allowed-length-of-one-hundred-characters-and-should-cause-a-validation-error",
			},
			expectError: true,
		},
		{
			name: "Term too long",
			utmTags: &UTMTags{
				Term: "this-is-a-very-long-term-name-that-exceeds-the-maximum-allowed-length-of-one-hundred-characters-and-should-cause-a-validation-error",
			},
			expectError: true,
		},
		{
			name: "Content too long",
			utmTags: &UTMTags{
				Content: "this-is-a-very-long-content-name-that-exceeds-the-maximum-allowed-length-of-one-hundred-characters-and-should-cause-a-validation-error",
			},
			expectError: true,
		},
		{
			name: "Contains control character",
			utmTags: &UTMTags{
				Source: "google\x00",
			},
			expectError: true,
		},
		{
			name: "Contains newline character",
			utmTags: &UTMTags{
				Source: "google\n",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.utmTags.Validate()
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateUTMValue(t *testing.T) {
	tests := []struct {
		name        string
		fieldName   string
		value       string
		expectError bool
	}{
		{
			name:        "Empty value",
			fieldName:   "source",
			value:       "",
			expectError: false,
		},
		{
			name:        "Valid value",
			fieldName:   "source",
			value:       "google",
			expectError: false,
		},
		{
			name:        "Valid value with spaces",
			fieldName:   "term",
			value:       "running shoes",
			expectError: false,
		},
		{
			name:        "Valid value with special characters",
			fieldName:   "campaign",
			value:       "summer-sale_2024",
			expectError: false,
		},
		{
			name:        "Value too long",
			fieldName:   "source",
			value:       "this-is-a-very-long-source-name-that-exceeds-the-maximum-allowed-length-of-one-hundred-characters-and-should-cause-a-validation-error",
			expectError: true,
		},
		{
			name:        "Contains null byte",
			fieldName:   "source",
			value:       "google\x00",
			expectError: true,
		},
		{
			name:        "Contains newline",
			fieldName:   "source",
			value:       "google\n",
			expectError: true,
		},
		{
			name:        "Contains tab",
			fieldName:   "source",
			value:       "google\t",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUTMValue(tt.fieldName, tt.value)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}
