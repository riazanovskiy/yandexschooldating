package util

import (
	"context"
	"strings"
	"time"

	"yandexschooldating/config"
	"yandexschooldating/reminder"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func GetMongoClient(ctx context.Context, uri string, timeout time.Duration) (*mongo.Client, error) {
	clientOptions := options.Client().ApplyURI(uri).SetServerSelectionTimeout(timeout)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func DropTestDatabaseOrPanic(ctx context.Context, client *mongo.Client, database string) {
	if !strings.HasPrefix(database, "test") {
		panic("DropTestDatabase called on " + database)
	}
	err := client.Database(database).Drop(ctx)
	if err != nil {
		panic(err)
	}
}

func IsChannelEmpty(channel chan reminder.Reminder) bool {
	select {
	case _, ok := <-channel:
		return !ok
	default:
		return true
	}
}

func GetLocationForCityOrUTC(city string) *time.Location {
	location, ok := config.CitiesLocation[city]
	if ok {
		return location
	}
	return time.UTC
}
