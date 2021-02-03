package user_test

import (
	"context"
	"testing"
	"time"

	"yandexschooldating/config"
	"yandexschooldating/user"
	"yandexschooldating/util"

	"github.com/stretchr/testify/require"
)

func TestDao(t *testing.T) {
	ctx := context.Background()
	client, err := util.GetMongoClient(ctx, config.MongoUri, 2*time.Second)
	if err != nil {
		panic(err)
	}
	dao := user.NewDAO(client, "test")

	err = dao.UpsertUser(ctx, 1, "durov", "Dubai", 1, true)
	require.Nil(t, err)

	err = dao.UpsertUser(ctx, 2, "nikolai", "Dubai", 2, false)
	require.Nil(t, err)

	nonExisting, err := dao.FindUserByID(ctx, 5)
	require.Nil(t, err)
	require.Nil(t, nonExisting)

	nikolai, err := dao.FindUserByID(ctx, 2)
	require.Nil(t, err)
	require.NotNil(t, nikolai)
	require.Equal(t, "nikolai", nikolai.Username)

	active, err := dao.FindActiveUsers(ctx)
	require.Nil(t, err)
	require.Len(t, active, 1)
	require.Equal(t, 1, active[0].ID)
	require.Equal(t, "durov", active[0].Username)

	err = client.Disconnect(ctx)
	if err != nil {
		panic(err)
	}

	_, err = dao.FindUserByID(ctx, 2)
	require.NotNil(t, err)
	_, err = dao.FindActiveUsers(ctx)
	require.NotNil(t, err)
}
