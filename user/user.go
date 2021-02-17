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

type DAO struct {
	users *mongo.Collection
}

func NewDAO(client *mongo.Client, database string) *DAO {
	return &DAO{users: client.Database(database).Collection("users")}
}

func (m *DAO) FindActiveUsers(ctx context.Context) ([]User, error) {
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

func (m *DAO) FindUserByID(ctx context.Context, ID int) (*User, error) {
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

func (m *DAO) UpsertUser(ctx context.Context, ID int, username, city string, chatID int64, active bool) error {
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

func (m *DAO) UpdateActiveStatus(ctx context.Context, ID int, active bool) error {
	result, err := m.users.UpdateOne(ctx, bson.M{UserBSON.ID: ID}, bson.M{"$set": bson.M{UserBSON.Active: active}})
	if err != nil {
		return errorx.Decorate(err, "error updating active status for user %d", ID)
	}
	if result.MatchedCount == 0 {
		return errorx.IllegalArgument.New("error updating active status: user %d not found", ID)
	}
	return nil
}
