package redirect

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

func TestRedirectLookupService(t *testing.T) {
	// This is a basic test that doesn't require a real database
	// In a real test environment, you'd use a test database

	// Create a mock database (nil for this test)
	var db *mongo.Database
	service := NewRedirectLookupService(db, 10, time.Minute)

	// Test cache stats
	stats := service.GetCacheStats()
	if stats["capacity"] != 10 {
		t.Errorf("Expected capacity 10, got %v", stats["capacity"])
	}

	// Test cache operations
	service.ClearCache()
	if service.cache.Size() != 0 {
		t.Error("Expected cache to be empty after clear")
	}
}
