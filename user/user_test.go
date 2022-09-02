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

	err = dao.UpsertUser(ctx, 1, "durov", "Dubai", 1, true, true)
	require.NoError(t, err)

	err = dao.UpsertUser(ctx, 2, "nikolai", "Dubai", 2, false, false)
	require.NoError(t, err)

	nonExisting, err := dao.FindUserByID(ctx, 5)
	require.NoError(t, err)
	require.Nil(t, nonExisting)

	nikolai, err := dao.FindUserByID(ctx, 2)
	require.NoError(t, err)
	require.NotNil(t, nikolai)
	require.Equal(t, "nikolai", nikolai.Username)

	active, err := dao.FindActiveUsers(ctx)
	require.NoError(t, err)
	require.Len(t, active, 1)
	require.Equal(t, 1, active[0].ID)
	require.Equal(t, "durov", active[0].Username)

	err = dao.UpdateActiveStatus(ctx, 2, true)
	require.NoError(t, err)
	active, err = dao.FindActiveUsers(ctx)
	require.NoError(t, err)
	require.Len(t, active, 2)

	err = dao.UpdateActiveStatus(ctx, 1, false)
	require.NoError(t, err)
	active, err = dao.FindActiveUsers(ctx)
	require.NoError(t, err)
	require.Len(t, active, 1)

	err = dao.UpdateActiveStatus(ctx, 88, true)
	require.Error(t, err)

	err = client.Disconnect(ctx)
	if err != nil {
		panic(err)
	}

	_, err = dao.FindUserByID(ctx, 2)
	require.Error(t, err)
	_, err = dao.FindActiveUsers(ctx)
	require.Error(t, err)

	err = dao.UpdateActiveStatus(ctx, 1, false)
	require.Error(t, err)
}
