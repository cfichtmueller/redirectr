package redirect

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cfichtmueller/redirectr/internal/ec"
	"github.com/cfichtmueller/redirectr/internal/infra/auth"
	"github.com/cfichtmueller/redirectr/internal/util"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	RedirectStatusActive   = "active"
	RedirectStatusInactive = "inactive"
	RedirectStatusDeleted  = "deleted"
)

var Statuses = []string{
	RedirectStatusActive,
	RedirectStatusInactive,
	RedirectStatusDeleted,
}

const (
	RedirectType301 = "301"
	RedirectType302 = "302"
)

var RedirectTypes = []string{
	RedirectType301,
	RedirectType302,
}

//
// Commands
//

type CreateCommand struct {
	SourceDomain string
	TargetDomain string
	Status       string
	RedirectType string
	UTMTags      *UTMTags
}

type UpdateCommand struct {
	SourceDomain string
	TargetDomain string
	Status       string
	RedirectType string
	UTMTags      *UTMTags
}

//
// Model
//

// UTMTags represents UTM tracking parameters for redirects
type UTMTags struct {
	Source   string `bson:"source,omitempty"`   // utm_source
	Medium   string `bson:"medium,omitempty"`   // utm_medium
	Campaign string `bson:"campaign,omitempty"` // utm_campaign
	Term     string `bson:"term,omitempty"`     // utm_term
	Content  string `bson:"content,omitempty"`  // utm_content
}

// HasUTMTags returns true if any UTM tag is configured
func (u *UTMTags) HasUTMTags() bool {
	if u == nil {
		return false
	}
	return u.Source != "" || u.Medium != "" || u.Campaign != "" || u.Term != "" || u.Content != ""
}

// Validate validates UTM tag values
func (u *UTMTags) Validate() error {
	if u == nil {
		return nil
	}

	// Validate each UTM tag value
	if err := validateUTMValue("source", u.Source); err != nil {
		return err
	}
	if err := validateUTMValue("medium", u.Medium); err != nil {
		return err
	}
	if err := validateUTMValue("campaign", u.Campaign); err != nil {
		return err
	}
	if err := validateUTMValue("term", u.Term); err != nil {
		return err
	}
	if err := validateUTMValue("content", u.Content); err != nil {
		return err
	}

	return nil
}

// validateUTMValue validates a single UTM parameter value
func validateUTMValue(fieldName, value string) error {
	if value == "" {
		return nil // Empty values are allowed
	}

	// Check length (UTM parameters should be reasonable length)
	if len(value) > 100 {
		return fmt.Errorf("utm_%s too long: maximum 100 characters allowed", fieldName)
	}

	// Check for invalid characters (basic validation)
	for _, char := range value {
		if char < 32 || char == 127 {
			return fmt.Errorf("utm_%s contains invalid character", fieldName)
		}
	}

	return nil
}

type Redirect struct {
	ID           string    `bson:"_id"`
	UserID       string    `bson:"userId"`
	SourceDomain string    `bson:"sourceDomain"`
	TargetDomain string    `bson:"targetDomain"`
	Status       string    `bson:"status"`
	RedirectType string    `bson:"redirectType"`
	UTMTags      *UTMTags  `bson:"utmTags,omitempty"`
	CreatedAt    time.Time `bson:"createdAt"`
	CreatedBy    string    `bson:"createdBy"`
	UpdatedAt    time.Time `bson:"updatedAt"`
	UpdatedBy    string    `bson:"updatedBy"`
	ETag         string    `bson:"etag"`
}

func (r *Redirect) IsActive() bool {
	return r.Status == RedirectStatusActive
}

func (r *Redirect) IsCircular() bool {
	return strings.EqualFold(r.SourceDomain, r.TargetDomain)
}

// CheckForCircularRedirect checks if creating a redirect from sourceDomain to targetDomain
// would create a circular redirect chain. It performs a depth-first search to detect cycles.
func CheckForCircularRedirect(ctx context.Context, sourceDomain, targetDomain string) error {
	// First check for direct circular redirect
	if strings.EqualFold(sourceDomain, targetDomain) {
		return ec.CircularRedirectNotAllowed
	}

	// Check for indirect circular redirects by following the redirect chain
	visited := make(map[string]bool)
	return checkCircularChain(ctx, targetDomain, sourceDomain, visited)
}

// checkCircularChain recursively checks if following redirects from startDomain
// would eventually lead back to originalDomain, creating a cycle.
func checkCircularChain(ctx context.Context, startDomain, originalDomain string, visited map[string]bool) error {
	// If we've already visited this domain in this chain, we have a cycle
	if visited[startDomain] {
		return ec.CircularRedirectNotAllowed
	}

	// Mark current domain as visited
	visited[startDomain] = true
	defer func() {
		// Clean up visited map for this path
		delete(visited, startDomain)
	}()

	// If we've reached the original domain, we have a cycle
	if strings.EqualFold(startDomain, originalDomain) {
		return ec.CircularRedirectNotAllowed
	}

	// Look for any active redirects from this domain
	redirect, err := FindBySourceDomain(ctx, startDomain)
	if err != nil {
		// If no redirect found, this path ends here (no cycle)
		if err == ec.NoSuchRedirect {
			return nil
		}
		// If it's a different error (like database connection issue),
		// we can't determine if there's a cycle, so we'll be conservative
		// and allow the redirect (this prevents blocking legitimate redirects
		// when there are temporary database issues)
		return nil
	}

	// Recursively check the target domain
	return checkCircularChain(ctx, redirect.TargetDomain, originalDomain, visited)
}

//
// Filter
//

type Filter struct {
	ID           string
	IDs          []string
	UserID       string
	SourceDomain string
	TargetDomain string
	Status       string
	Statuses     []string
	Q            string
	Limit        int
	Offset       int
}

func (f *Filter) bson() bson.D {
	d := util.NewFilter().
		AppendIf(f.ID != "", "_id", f.ID).
		AppendIf(f.IDs != nil, "_id", bson.M{"$in": f.IDs}).
		AppendIf(f.UserID != "", "userId", f.UserID).
		AppendIf(f.SourceDomain != "", "sourceDomain", strings.ToLower(f.SourceDomain)).
		AppendIf(f.TargetDomain != "", "targetDomain", strings.ToLower(f.TargetDomain)).
		AppendIf(f.Status != "", "status", f.Status).
		AppendIf(f.Statuses != nil, "status", bson.M{"$in": f.Statuses}).
		Bson()

	if f.Q != "" {
		prefixRegex := bson.Regex{Pattern: "^" + f.Q, Options: "i"}
		orFilter := bson.E{
			Key: "$or", Value: bson.A{
				bson.D{{Key: "sourceDomain", Value: prefixRegex}},
				bson.D{{Key: "targetDomain", Value: prefixRegex}},
			},
		}
		d = append(d, orFilter)
	}

	return d
}

//
// Methods
//

func Count(ctx context.Context, filter *Filter) (int64, error) {
	count, err := redirectCollection.CountDocuments(ctx, filter.bson())
	if err != nil {
		return 0, fmt.Errorf("unable to count redirects: %w", err)
	}
	return count, nil
}

func FindMany(ctx context.Context, filter *Filter) ([]*Redirect, error) {
	opts := options.Find().SetSort(bson.D{
		{Key: "createdAt", Value: -1},
	})

	if filter.Limit > 0 {
		opts.SetLimit(int64(filter.Limit))
	}
	if filter.Offset > 0 {
		opts.SetSkip(int64(filter.Offset))
	}

	return findMany(ctx, filter.bson(), opts)
}

func FindOne(ctx context.Context, filter *Filter) (*Redirect, error) {
	return findOne(ctx, filter.bson())
}

func Create(ctx context.Context, principal *auth.Principal, cmd CreateCommand) (*Redirect, error) {
	now := time.Now()
	sourceDomain := strings.ToLower(cmd.SourceDomain)
	targetDomain := strings.ToLower(cmd.TargetDomain)

	// Check for duplicate source domain
	existingCount, err := redirectCollection.CountDocuments(ctx, bson.M{
		"sourceDomain": sourceDomain,
		"status":       bson.M{"$ne": RedirectStatusDeleted},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to check for existing redirect: %w", err)
	}
	if existingCount > 0 {
		return nil, ec.SourceDomainNotAvailable
	}

	// Check for circular redirect (both direct and indirect)
	if err := CheckForCircularRedirect(ctx, sourceDomain, targetDomain); err != nil {
		return nil, err
	}

	// Validate UTM tags if provided
	if cmd.UTMTags != nil {
		if err := cmd.UTMTags.Validate(); err != nil {
			return nil, fmt.Errorf("invalid UTM tags: %w", err)
		}
	}

	r := &Redirect{
		ID:           util.RandomId(),
		UserID:       principal.ID,
		SourceDomain: sourceDomain,
		TargetDomain: targetDomain,
		Status:       cmd.Status,
		RedirectType: cmd.RedirectType,
		UTMTags:      cmd.UTMTags,
		CreatedAt:    now,
		CreatedBy:    principal.Urn,
		UpdatedAt:    now,
		UpdatedBy:    principal.Urn,
		ETag:         util.NewEtag(),
	}

	if _, err := redirectCollection.InsertOne(ctx, r); err != nil {
		return nil, fmt.Errorf("unable to save redirect: %w", err)
	}

	// Cache the new redirect for immediate availability
	if lookupService != nil {
		lookupService.InvalidateCache(r.SourceDomain) // This will clear any negative cache entries
	}

	return r, nil
}

func Save(ctx context.Context, r *Redirect) error {
	updateDoc := bson.M{
		"sourceDomain": r.SourceDomain,
		"targetDomain": r.TargetDomain,
		"status":       r.Status,
		"redirectType": r.RedirectType,
		"updatedAt":    r.UpdatedAt,
		"updatedBy":    r.UpdatedBy,
		"etag":         r.ETag,
	}

	// Include UTM tags if they exist
	if r.UTMTags != nil {
		updateDoc["utmTags"] = r.UTMTags
	}

	if _, err := redirectCollection.UpdateOne(ctx, bson.M{"_id": r.ID}, bson.M{"$set": updateDoc}); err != nil {
		return fmt.Errorf("unable to save redirect: %w", err)
	}

	// Invalidate cache for this redirect
	if lookupService != nil {
		lookupService.InvalidateCache(r.SourceDomain)
	}

	return nil
}

func Update(ctx context.Context, principal *auth.Principal, r *Redirect, cmd UpdateCommand) (*Redirect, error) {
	now := time.Now()
	sourceDomain := strings.ToLower(cmd.SourceDomain)
	targetDomain := strings.ToLower(cmd.TargetDomain)

	// Check for duplicate source domain (excluding current redirect)
	existingCount, err := redirectCollection.CountDocuments(ctx, bson.M{
		"sourceDomain": sourceDomain,
		"_id":          bson.M{"$ne": r.ID},
		"status":       bson.M{"$ne": RedirectStatusDeleted},
	})
	if err != nil {
		return nil, fmt.Errorf("unable to check for existing redirect: %w", err)
	}
	if existingCount > 0 {
		return nil, ec.SourceDomainNotAvailable
	}

	// Check for circular redirect (both direct and indirect)
	if err := CheckForCircularRedirect(ctx, sourceDomain, targetDomain); err != nil {
		return nil, err
	}

	// Validate UTM tags if provided
	if cmd.UTMTags != nil {
		if err := cmd.UTMTags.Validate(); err != nil {
			return nil, fmt.Errorf("invalid UTM tags: %w", err)
		}
	}

	r.SourceDomain = sourceDomain
	r.TargetDomain = targetDomain
	r.Status = cmd.Status
	r.RedirectType = cmd.RedirectType
	r.UTMTags = cmd.UTMTags
	r.UpdatedAt = now
	r.UpdatedBy = principal.Urn
	r.ETag = util.NewEtag()

	if err := Save(ctx, r); err != nil {
		return nil, err
	}

	return r, nil
}

func Delete(ctx context.Context, principal *auth.Principal, id string) error {
	r, err := findOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}

	// Soft delete by setting status to deleted
	r.Status = RedirectStatusDeleted
	r.UpdatedAt = time.Now()
	r.UpdatedBy = principal.Urn
	r.ETag = util.NewEtag()

	// Save the updated redirect (this will also invalidate cache via Save function)
	return Save(ctx, r)
}

func FindBySourceDomain(ctx context.Context, sourceDomain string) (*Redirect, error) {
	return findOne(ctx, bson.M{
		"sourceDomain": strings.ToLower(sourceDomain),
		"status":       RedirectStatusActive,
	})
}

func findMany(ctx context.Context, filter bson.D, opts ...options.Lister[options.FindOptions]) ([]*Redirect, error) {
	cur, err := redirectCollection.Find(ctx, filter, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to find redirects: %w", err)
	}
	defer cur.Close(ctx)

	result := make([]*Redirect, 0, cur.RemainingBatchLength())
	for cur.Next(ctx) {
		var current Redirect
		if err := cur.Decode(&current); err != nil {
			return nil, fmt.Errorf("unable to decode redirect: %w", err)
		}
		result = append(result, &current)
	}
	return result, nil
}

func findOne(ctx context.Context, filter interface{}) (*Redirect, error) {
	var result Redirect
	if err := redirectCollection.FindOne(ctx, filter).Decode(&result); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ec.NoSuchRedirect
		}
		return nil, fmt.Errorf("unable to find redirect: %w", err)
	}
	return &result, nil
}
