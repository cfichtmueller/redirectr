package iam

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cfichtmueller/redirectr/internal/ec"
	"github.com/cfichtmueller/redirectr/internal/infra/auth"
	"github.com/cfichtmueller/redirectr/internal/util"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/crypto/bcrypt"
)

//
// Commands
//

type CreateUserCommand struct {
	Email    string
	Password string
}

//
// Model
//

type User struct {
	ID           string    `bson:"_id"`
	Email        string    `bson:"email"`
	PasswordHash []byte    `bson:"passwordHash"`
	CreatedAt    time.Time `bson:"createdAt"`
}

func (u *User) SetPassword(password string) error {
	pwHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = pwHash
	return nil
}

func (u *User) PasswordMatches(plainTextPassword string) bool {
	return bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(plainTextPassword)) == nil
}

//
// Methods
//

func UserUrn(id string) string {
	return "staff:user:" + id
}

func CountUsers(ctx context.Context) (int64, error) {
	count, err := userCollection.CountDocuments(ctx, bson.D{})
	if err != nil {
		return 0, fmt.Errorf("unable to count users: %w", err)
	}
	return count, nil
}

func FindUserByID(ctx context.Context, id string) (*User, error) {
	return findOneUser(ctx, bson.M{"_id": id})
}

func FindUserByEmail(ctx context.Context, email string) (*User, error) {
	return findOneUser(ctx, bson.M{"email": strings.ToLower(email)})
}

func CreateUser(ctx context.Context, cmd CreateUserCommand) (*User, error) {
	u := &User{
		ID:        util.RandomId(),
		Email:     strings.ToLower(cmd.Email),
		CreatedAt: time.Now(),
	}
	if err := u.SetPassword(cmd.Password); err != nil {
		return nil, err
	}

	if _, err := userCollection.InsertOne(ctx, u); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, ec.EmailNotAvailable
		}
		return nil, fmt.Errorf("unable to save user: %w", err)
	}

	return u, nil
}

func GetPrincipal(ctx context.Context, id string) (*auth.Principal, error) {
	user, err := FindUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, ec.NoSuchUser) {
			return nil, ec.InvalidCredentials
		}
		return nil, err
	}
	return principalFromUser(user), nil
}

func principalFromUser(u *User) *auth.Principal {
	return &auth.Principal{
		ID:           u.ID,
		Urn:          UserUrn(u.ID),
		EventContext: "staff",
	}
}

func findOneUser(ctx context.Context, filter interface{}) (*User, error) {
	var result User
	if err := userCollection.FindOne(ctx, filter).Decode(&result); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ec.NoSuchUser
		}
		return nil, fmt.Errorf("unable to find user: %w", err)
	}
	return &result, nil
}
