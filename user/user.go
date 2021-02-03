package user

import (
	"context"

	"github.com/joomcode/errorx"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type User struct {
	ID          int    `bson:"_id"`
	Username    string `bson:"username"`
	City        string `bson:"city"`
	ChatID      int64  `bson:"chatId"`
	Active      bool   `bson:"active"`
	RemoteFirst bool   `bson:"remoteFirst"`
}

//goland:noinspection GoNameStartsWithPackageName
var UserBSON = struct {
	ID          string
	Username    string
	City        string
	ChatID      string
	Active      string
	RemoteFirst string
}{"_id", "username", "city", "chatId", "active", "remoteFirst"}

type DAO interface {
	UpsertUser(ctx context.Context, ID int, username, city string, chatID int64, active bool) error
	FindUserByID(ctx context.Context, ID int) (*User, error)
	FindActiveUsers(ctx context.Context) ([]User, error)
}

type dao struct {
	users *mongo.Collection
}

var _ DAO = (*dao)(nil)

func NewDAO(client *mongo.Client, database string) DAO {
	return &dao{users: client.Database(database).Collection("users")}
}

func (m *dao) FindActiveUsers(ctx context.Context) ([]User, error) {
	cursor, err := m.users.Find(ctx, bson.M{UserBSON.Active: true})
	if err != nil {
		return nil, errorx.Decorate(err, "error finding active users")
	}
	var result []User
	for cursor.Next(ctx) {
		var user User
		err = cursor.Decode(&user)
		if err != nil {
			return nil, errorx.Decorate(err, "can't decode user")
		}
		result = append(result, user)
	}
	return result, nil
}

func (m *dao) FindUserByID(ctx context.Context, ID int) (*User, error) {
	result := m.users.FindOne(ctx, bson.M{UserBSON.ID: ID})
	if result.Err() == mongo.ErrNoDocuments {
		return nil, nil
	}

	if result.Err() != nil {
		return nil, errorx.Decorate(result.Err(), "can't find the user for id %d", ID)
	}

	var user User
	err := result.Decode(&user)
	if err != nil {
		return nil, errorx.Decorate(err, "can't decode user")
	}

	return &user, nil
}

func (m *dao) UpsertUser(ctx context.Context, ID int, username, city string, chatID int64, active bool) error {
	user := User{
		ID:          ID,
		Username:    username,
		City:        city,
		ChatID:      chatID,
		Active:      active,
		RemoteFirst: false,
	}
	_, err := m.users.UpdateOne(ctx, bson.M{UserBSON.ID: ID}, bson.M{"$set": user}, options.Update().SetUpsert(true))
	return err
}
