package redirect

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cfichtmueller/redirectr/internal/util/cache"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func WarmCache(ctx context.Context, limit int) error {
	return lookupService.WarmCache(ctx, limit)
}

func Lookup(ctx context.Context, sourceDomain string) (*RedirectResult, error) {
	return lookupService.LookupRedirect(ctx, sourceDomain)
}

func GetCacheStats() map[string]interface{} {
	return lookupService.GetCacheStats()
}

func ClearCache() {
	lookupService.ClearCache()
}

// LookupService provides fast redirect lookups with caching
type LookupService struct {
	cache    *cache.LRUCache
	db       *mongo.Database
	cacheTTL time.Duration
}

// RedirectResult represents the result of a redirect lookup
type RedirectResult struct {
	TargetDomain string
	Found        bool
	FromCache    bool
	RedirectID   string
	UserID       string
	RedirectType string
	UTMTags      *UTMTags
}

// NewRedirectLookupService creates a new redirect lookup service
func NewRedirectLookupService(db *mongo.Database, cacheCapacity int, cacheTTL time.Duration) *LookupService {
	return &LookupService{
		cache:    cache.NewLRUCache(cacheCapacity),
		db:       db,
		cacheTTL: cacheTTL,
	}
}

// LookupRedirect performs a fast redirect lookup with cache fallback to MongoDB
func (s *LookupService) LookupRedirect(ctx context.Context, sourceDomain string) (*RedirectResult, error) {
	// Normalize domain to lowercase for consistent lookups
	normalizedDomain := strings.ToLower(strings.TrimSpace(sourceDomain))

	// Try cache first
	if entry, found := s.cache.Get(normalizedDomain); found {
		// Check if cache entry is still valid
		if time.Since(entry.CachedAt) < s.cacheTTL {
			var utmTags *UTMTags
			if entry.UTMTags != nil {
				utmTags = entry.UTMTags.(*UTMTags)
			}
			return &RedirectResult{
				TargetDomain: entry.TargetDomain,
				Found:        true,
				FromCache:    true,
				RedirectID:   entry.RedirectID,
				UserID:       entry.UserID,
				RedirectType: entry.RedirectType,
				UTMTags:      utmTags,
			}, nil
		}
		// Cache entry expired, remove it
		s.cache.Delete(normalizedDomain)
	}

	// Cache miss or expired - query MongoDB
	redirect, err := FindBySourceDomain(ctx, normalizedDomain)
	if err != nil {
		// If it's a "not found" error, cache the negative result briefly
		if err == mongo.ErrNoDocuments {
			s.cacheNegativeResult(normalizedDomain)
			return &RedirectResult{
				Found:     false,
				FromCache: false,
			}, nil
		}
		return nil, fmt.Errorf("failed to lookup redirect from database: %w", err)
	}

	// Cache the positive result
	s.cache.Put(normalizedDomain, &cache.CacheEntry{
		TargetDomain: redirect.TargetDomain,
		RedirectID:   redirect.ID,
		UserID:       redirect.UserID,
		RedirectType: redirect.RedirectType,
		UTMTags:      redirect.UTMTags,
		CachedAt:     time.Now(),
	})

	return &RedirectResult{
		TargetDomain: redirect.TargetDomain,
		Found:        true,
		FromCache:    false,
		RedirectID:   redirect.ID,
		UserID:       redirect.UserID,
		RedirectType: redirect.RedirectType,
		UTMTags:      redirect.UTMTags,
	}, nil
}

// cacheNegativeResult caches a negative result for a short time to avoid repeated DB queries
func (s *LookupService) cacheNegativeResult(domain string) {
	// Cache negative results for a shorter time to allow for quick updates
	negativeTTL := s.cacheTTL / 4
	if negativeTTL < time.Minute {
		negativeTTL = time.Minute
	}

	s.cache.Put(domain, &cache.CacheEntry{
		TargetDomain: "",                                        // Empty target indicates not found
		RedirectID:   "",                                        // Empty redirect ID indicates not found
		UserID:       "",                                        // Empty user ID indicates not found
		CachedAt:     time.Now().Add(-s.cacheTTL + negativeTTL), // Expire sooner
	})
}

// InvalidateCache removes a specific domain from the cache
func (s *LookupService) InvalidateCache(sourceDomain string) {
	normalizedDomain := strings.ToLower(strings.TrimSpace(sourceDomain))
	s.cache.Delete(normalizedDomain)
	slog.Debug("invalidated cache entry", "domain", normalizedDomain)
}

// ClearCache removes all entries from the cache
func (s *LookupService) ClearCache() {
	s.cache.Clear()
	slog.Info("cleared redirect cache")
}

// GetCacheStats returns cache statistics
func (s *LookupService) GetCacheStats() map[string]interface{} {
	return map[string]interface{}{
		"size":     s.cache.Size(),
		"capacity": s.cache.GetCapacity(),
		"ttl":      s.cacheTTL.String(),
	}
}

// WarmCache preloads frequently accessed redirects into the cache
func (s *LookupService) WarmCache(ctx context.Context, limit int) error {
	slog.Info("warming redirect cache", "limit", limit)

	// Get recent active redirects to warm the cache
	filter := &Filter{
		Status: RedirectStatusActive,
		Limit:  limit,
	}

	redirects, err := FindMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to fetch redirects for cache warming: %w", err)
	}

	warmed := 0
	for _, r := range redirects {
		s.cache.Put(r.SourceDomain, &cache.CacheEntry{
			TargetDomain: r.TargetDomain,
			RedirectID:   r.ID,
			UserID:       r.UserID,
			UTMTags:      r.UTMTags,
			CachedAt:     time.Now(),
		})
		warmed++
	}

	slog.Info("cache warming completed", "warmed", warmed)
	return nil
}
