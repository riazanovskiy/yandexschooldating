package util_test

import (
	"context"
	"testing"
	"time"

	"yandexschooldating/config"
	"yandexschooldating/reminder"
	"yandexschooldating/util"

	"github.com/stretchr/testify/require"
)

func TestGetMongoClient(t *testing.T) {
	ctx := context.Background()
	_, err := util.GetMongoClient(ctx, config.MongoUri, 2*time.Second)
	require.Nil(t, err)

	_, err = util.GetMongoClient(ctx, "https://mongo:27017", 2*time.Second)
	require.NotNil(t, err)

	_, err = util.GetMongoClient(ctx, "mongodb://localhost:666", 2*time.Second)
	require.NotNil(t, err)
}

func TestDropTestDatabaseOrPanic(t *testing.T) {
	ctx := context.Background()
	client, err := util.GetMongoClient(ctx, config.MongoUri, 2*time.Second)
	if err != nil {
		panic(err)
	}

	require.Panics(t, func() {
		util.DropTestDatabaseOrPanic(ctx, client, "main_not_a_real")
	})
	err = client.Disconnect(ctx)
	if err != nil {
		panic(err)
	}
	require.Panics(t, func() {
		util.DropTestDatabaseOrPanic(ctx, client, "test_not_a_real")
	})
}

func TestIsChannelEmpty(t *testing.T) {
	channel := make(chan reminder.Reminder, 1)
	start := make(chan struct{})
	end := make(chan struct{})
	go func() {
		<-start
		require.False(t, util.IsChannelEmpty(channel))
		require.True(t, util.IsChannelEmpty(channel))
		close(channel)
		require.True(t, util.IsChannelEmpty(channel))
		end <- struct{}{}
	}()
	channel <- reminder.Reminder{}
	start <- struct{}{}
	<-end
}
