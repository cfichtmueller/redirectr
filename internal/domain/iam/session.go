package iam

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cfichtmueller/redirectr/internal/util"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Session struct {
	ID        string    `bson:"_id"`
	User      string    `bson:"user"`
	CreatedAt time.Time `bson:"createdAt"`
	ExpiresAt time.Time `bson:"expiresAt"`
}

func (s *Session) IsExpired() bool {
	return s.ExpiresAt.Before(time.Now())
}

var (
	SessionTTL         = time.Hour
	ErrSessionNotFound = fmt.Errorf("session not found")
)

func CreateSession(ctx context.Context, userID string) (*Session, error) {
	now := util.TimeNow()
	s := &Session{
		ID:        util.NewId(64),
		User:      userID,
		CreatedAt: now,
		ExpiresAt: now.Add(SessionTTL),
	}

	if _, err := sessionCollection.InsertOne(ctx, s); err != nil {
		return nil, fmt.Errorf("unable to create session record: %w", err)
	}

	return s, nil
}

func GetSession(ctx context.Context, id string) (*Session, error) {
	return findOneSession(ctx, bson.M{"_id": id})
}

func findOneSession(ctx context.Context, filter interface{}) (*Session, error) {
	var s Session
	if err := sessionCollection.FindOne(ctx, filter).Decode(&s); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("unable to find session: %w", err)
	}
	return &s, nil
}

func DeleteSession(ctx context.Context, id string) error {
	if _, err := sessionCollection.DeleteOne(ctx, bson.M{"_id": id}); err != nil {
		return fmt.Errorf("unable to delete session: %w", err)
	}
	return nil
}
