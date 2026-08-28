package audit

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/cfichtmueller/redirectr/internal/infra/auth"
	"github.com/cfichtmueller/redirectr/internal/util"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Data map[string]any
type EventType string
type Tags map[string]string

type Event struct {
	ID        string    `bson:"_id"`
	Context   string    `bson:"context"`
	Timestamp time.Time `bson:"timestamp"`
	Type      string    `bson:"type"`
	Entity    string    `bson:"entity,omitempty"`
	Principal string    `bson:"principal"`
	Tags      Tags      `bson:"tags"`
	Data      Data      `bson:"data"`
}

func newEvent(principal *auth.Principal, eventType EventType, tags Tags) *Event {
	if len(principal.EventContext) == 0 || principal == nil || len(eventType) == 0 {
		panic("context, principal, eventType are mandatory")
	}
	return &Event{
		ID:        util.RandomId(),
		Context:   principal.EventContext,
		Timestamp: util.TimeNow(),
		Type:      string(eventType),
		Entity:    "",
		Principal: principal.Urn,
		Tags:      tags,
		Data:      make(Data),
	}
}

var (
	collection *mongo.Collection
)

func Configure(db *mongo.Database) {
	collection = db.Collection("audit_events")
	if _, err := collection.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{Keys: bson.D{{Key: "context", Value: 1}, {Key: "type", Value: 1}, {Key: "timestamp", Value: 1}}},
		{Keys: bson.D{{Key: "context", Value: 1}, {Key: "entity", Value: 1}, {Key: "timestamp", Value: 1}}},
		{Keys: bson.D{{Key: "context", Value: 1}, {Key: "principal", Value: 1}, {Key: "timestamp", Value: 1}}},
		{Keys: bson.D{{Key: "tags.$**", Value: 1}}},
	}); err != nil {
		slog.Error("unable to create audit indices", "error", err)
	}
}

// WriteEntity writes an audit event with entity information
func WriteEntity(ctx context.Context, principal *auth.Principal, eventType EventType, entity string, tags Tags) error {
	return WriteEntityData(ctx, principal, eventType, entity, tags, map[string]any{})
}

// WriteEntityData writes an audit event with entity information
func WriteEntityData(ctx context.Context, principal *auth.Principal, eventType EventType, entity string, tags Tags, data Data) error {
	event := newEvent(principal, eventType, tags)
	event.Entity = entity
	event.Tags = tags
	event.Data = data
	return write(ctx, event)
}

func write(c context.Context, e *Event) error {
	if _, err := collection.InsertOne(c, e); err != nil {
		return fmt.Errorf("unable to write audit event: %w", err)
	}
	return nil
}
