package match_test

import (
	"context"
	"testing"
	"time"

	"yandexschooldating/clock"
	"yandexschooldating/config"
	"yandexschooldating/match"
	"yandexschooldating/util"

	"github.com/stretchr/testify/require"
)

func TestDao(t *testing.T) {
	ctx := context.Background()
	client, err := util.GetMongoClient(ctx, config.MongoUri, 2*time.Second)
	if err != nil {
		panic(err)
	}

	testDatabase := "test_matches"
	util.DropTestDatabaseOrPanic(ctx, client, testDatabase)

	start := time.Date(2020, 7, 5, 4, 20, 0, 0, time.UTC)
	clock := &clock.Fake{Current: start}

	dao := match.NewDAO(client, testDatabase, clock)
	err = dao.InitializeMatchingCycle(ctx)
	require.NoError(t, err)

	everyone, err := dao.GetAllMatchedUsers(ctx)
	require.NoError(t, err)
	require.Nil(t, everyone)

	err = dao.AddMatch(ctx, 2, 3)
	require.NoError(t, err)
	everyone, err = dao.GetAllMatchedUsers(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, everyone, []int{2, 3})

	clock.Current = clock.Current.AddDate(0, 0, 7)
	dao.IncrementMatchingCycle()
	err = dao.AddMatch(ctx, 2, 4)
	require.NoError(t, err)
	everyone, err = dao.GetAllMatchedUsers(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, everyone, []int{2, 4})

	clock.Current = clock.Current.AddDate(0, 0, 6).Add((60*20 + 31) * time.Minute)
	dao.IncrementMatchingCycle()
	err = dao.AddMatch(ctx, 2, 5)
	require.NoError(t, err)

	everyone, err = dao.GetAllMatchedUsers(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, everyone, []int{2, 5})

	clock.Current = clock.Current.AddDate(0, 0, 7).Add(31 * time.Minute)
	dao.IncrementMatchingCycle()
	err = dao.AddMatch(ctx, 2, 12)
	require.NoError(t, err)
	clock.Current = clock.Current.Add(5 * time.Minute)
	result, err := dao.FindCurrentMatchForUserID(ctx, 2)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 2, result.FirstID)
	require.Equal(t, 12, result.SecondID)
	result, err = dao.FindCurrentMatchForUserID(ctx, 12)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 12, result.FirstID)
	require.Equal(t, 2, result.SecondID)

	everyone, err = dao.GetAllMatchedUsers(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, everyone, []int{12, 2})

	result, err = dao.FindCurrentMatchForUserID(ctx, 3)
	require.NoError(t, err)
	require.Nil(t, result)

	result, err = dao.FindCurrentMatchForUserID(ctx, 5)
	require.NoError(t, err)
	require.Nil(t, result)

	dao = match.NewDAO(client, testDatabase, clock)
	err = dao.InitializeMatchingCycle(ctx)
	require.NoError(t, err)

	result, err = dao.FindCurrentMatchForUserID(ctx, 2)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 2, result.FirstID)
	require.Equal(t, 12, result.SecondID)
	result, err = dao.FindCurrentMatchForUserID(ctx, 12)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 12, result.FirstID)
	require.Equal(t, 2, result.SecondID)

	result, err = dao.FindCurrentMatchForUserID(ctx, 3)
	require.NoError(t, err)
	require.Nil(t, result)

	result, err = dao.FindCurrentMatchForUserID(ctx, 5)
	require.NoError(t, err)
	require.Nil(t, result)

	dao.IncrementMatchingCycle()
	result, err = dao.FindCurrentMatchForUserID(ctx, 2)
	require.NoError(t, err)
	require.Nil(t, result)

	result, err = dao.FindCurrentMatchForUserID(ctx, 12)
	require.NoError(t, err)
	require.Nil(t, result)

	err = dao.AddMatch(ctx, 85, 6)
	require.NoError(t, err)

	err = dao.AddMatch(ctx, 85, 7)
	require.Error(t, err)

	err = dao.AddMatch(ctx, 6, 85)
	require.Error(t, err)

	err = dao.AddMatch(ctx, 7, 85)
	require.Error(t, err)

	result, err = dao.FindCurrentMatchForUserID(ctx, 6)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 6, result.FirstID)
	require.Equal(t, 85, result.SecondID)
	require.Nil(t, result.MeetingTime)

	meetingTime := time.Date(2021, 1, 31, 23, 59, 56, 0, time.UTC)
	err = dao.UpdateMatchTime(ctx, 85, meetingTime)
	require.NoError(t, err)

	result, err = dao.FindCurrentMatchForUserID(ctx, 6)
	require.NoError(t, err)
	require.NotNil(t, result.MeetingTime)
	require.Equal(t, meetingTime.Unix(), result.MeetingTime.Unix())

	meetingTime = meetingTime.AddDate(0, 0, 1)
	err = dao.UpdateMatchTime(ctx, 6, meetingTime)
	require.NoError(t, err)

	result, err = dao.FindCurrentMatchForUserID(ctx, 85)
	require.NoError(t, err)
	require.NotNil(t, result.MeetingTime)
	require.Equal(t, meetingTime.Unix(), result.MeetingTime.Unix())

	err = dao.UpdateMatchTime(ctx, 1, meetingTime)
	require.Error(t, err)

	everyone, err = dao.GetAllMatchedUsers(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, everyone, []int{85, 6})

	err = dao.BreakMatchForUser(ctx, 85)
	require.NoError(t, err)

	everyone, err = dao.GetAllMatchedUsers(ctx)
	require.NoError(t, err)
	require.Nil(t, everyone)

	err = dao.BreakMatchForUser(ctx, 2)
	require.Error(t, err)

	err = dao.BreakMatchForUser(ctx, 12)
	require.Error(t, err)

	result, err = dao.FindCurrentMatchForUserID(ctx, 6)
	require.NoError(t, err)
	require.Nil(t, result)

	result, err = dao.FindCurrentMatchForUserID(ctx, 85)
	require.NoError(t, err)
	require.Nil(t, result)

	err = dao.AddMatch(ctx, 6, 100)
	require.NoError(t, err)

	err = dao.AddMatch(ctx, 85, 101)
	require.NoError(t, err)

	everyone, err = dao.GetAllMatchedUsers(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, everyone, []int{6, 85, 100, 101})

	err = dao.BreakMatchForUser(ctx, 101)
	require.NoError(t, err)

	err = dao.BreakMatchForUser(ctx, 100)
	require.NoError(t, err)

	err = dao.AddMatch(ctx, 6, 85)
	require.NoError(t, err)

	everyone, err = dao.GetAllMatchedUsers(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, everyone, []int{85, 6})

	result, err = dao.FindCurrentMatchForUserID(ctx, 6)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 6, result.FirstID)
	require.Equal(t, 85, result.SecondID)
	require.Nil(t, result.MeetingTime)

	err = client.Disconnect(ctx)
	if err != nil {
		panic(err)
	}

	err = dao.AddMatch(ctx, 1, 2)
	require.Error(t, err)

	_, err = dao.FindCurrentMatchForUserID(ctx, 6)
	require.Error(t, err)

	meetingTime = meetingTime.AddDate(0, 0, 1)
	err = dao.UpdateMatchTime(ctx, 6, meetingTime)
	require.Error(t, err)

	err = dao.BreakMatchForUser(ctx, 85)
	require.Error(t, err)

	_, err = dao.GetAllMatchedUsers(ctx)
	require.Error(t, err)

	dao = match.NewDAO(client, testDatabase, clock)
	err = dao.InitializeMatchingCycle(ctx)
	require.Error(t, err)
}
