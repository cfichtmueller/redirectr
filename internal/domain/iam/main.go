package iam

import (
	"context"
	"errors"
	"log/slog"

	"github.com/cfichtmueller/redirectr/internal/ec"
	"github.com/cfichtmueller/redirectr/internal/infra/auth"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	userCollection    *mongo.Collection
	sessionCollection *mongo.Collection
)

func Configure(db *mongo.Database) {
	userCollection = db.Collection("iam_users")
	sessionCollection = db.Collection("iam_sessions")

	if _, err := userCollection.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetName("idx_user_email").SetUnique(true),
	}); err != nil {
		slog.Error("unable to create user indices", "error", err)
	}
}

func AuthenticateUser(ctx context.Context, username, password string) (*auth.Principal, error) {
	user, err := FindUserByEmail(ctx, username)
	if err != nil {
		if errors.Is(err, ec.NoSuchUser) {
			return nil, ec.InvalidCredentials
		}
		return nil, err
	}
	if !user.PasswordMatches(password) {
		return nil, ec.InvalidCredentials
	}

	return principalFromUser(user), nil
}
