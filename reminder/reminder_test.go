package reminder_test

import (
	"context"
	"testing"
	"time"

	"yandexschooldating/clock"
	"yandexschooldating/config"
	"yandexschooldating/reminder"
	"yandexschooldating/util"

	"github.com/stretchr/testify/require"
)

func TestDao_AddReminder(t *testing.T) {
	ctx := context.Background()
	client, err := util.GetMongoClient(ctx, config.MongoUri, 2*time.Second)
	if err != nil {
		panic(err)
	}

	testDatabase := "test_reminders"
	util.DropTestDatabaseOrPanic(ctx, client, testDatabase)

	queue := make(chan reminder.Reminder)
	start := time.Now()
	clock := &clock.Fake{Current: start}

	dao := reminder.NewDAO(client, testDatabase, queue, clock)

	reminderTime := start.Add(time.Second * 4)
	err = dao.AddReminder(ctx, reminderTime, 42, "bring a towel")
	require.NoError(t, err)

	value, ok := <-queue
	elapsed := time.Now().Sub(start)
	require.True(t, ok)
	require.Equal(t, int64(42), value.ChatID)
	require.Equal(t, "bring a towel", value.Text)
	require.Equal(t, reminderTime.Unix(), value.UnixTime)
	require.LessOrEqual(t, (3 * time.Second).Nanoseconds(), elapsed.Nanoseconds())
	require.LessOrEqual(t, elapsed.Nanoseconds(), (5 * time.Second).Nanoseconds())

	require.True(t, util.IsChannelEmpty(queue))

	err = dao.AddReminder(ctx, time.Date(2020, 7, 5, 4, 20, 0, 0, time.UTC), 1, "will never see")
	require.Error(t, err)

	err = client.Disconnect(ctx)
	if err != nil {
		panic(err)
	}

	err = dao.AddReminder(ctx, start.Add(time.Hour), 1, "will never see")
	require.Error(t, err)
}

func TestDao_PopulateReminderQueue(t *testing.T) {
	ctx := context.Background()
	client, err := util.GetMongoClient(ctx, config.MongoUri, 2*time.Second)
	if err != nil {
		panic(err)
	}

	testDatabase := "test_reminders"
	util.DropTestDatabaseOrPanic(ctx, client, testDatabase)

	queue := make(chan reminder.Reminder)
	start := time.Now()
	clock := &clock.Fake{Current: start}

	dao := reminder.NewDAO(client, testDatabase, queue, clock)

	reminderTime := start.Add(time.Second * 4)
	err = dao.AddReminder(ctx, reminderTime, 42, "bring a towel")
	require.NoError(t, err)

	newQueue := make(chan reminder.Reminder)
	require.True(t, util.IsChannelEmpty(newQueue))
	clock.Current = start.Add(2 * time.Second)
	dao = reminder.NewDAO(client, testDatabase, newQueue, clock)
	require.NoError(t, dao.PopulateReminderQueue(ctx))

	value, ok := <-newQueue
	elapsed := time.Now().Sub(start)
	require.True(t, ok)
	require.Equal(t, int64(42), value.ChatID)
	require.Equal(t, "bring a towel", value.Text)
	require.Equal(t, reminderTime.Unix(), value.UnixTime)
	require.LessOrEqual(t, (1 * time.Second).Nanoseconds(), elapsed.Nanoseconds())
	require.LessOrEqual(t, elapsed.Nanoseconds(), (3 * time.Second).Nanoseconds())
	require.True(t, util.IsChannelEmpty(newQueue))

	newQueue = make(chan reminder.Reminder)
	require.True(t, util.IsChannelEmpty(newQueue))
	clock.Current = start.Add(5 * time.Second)
	dao = reminder.NewDAO(client, testDatabase, newQueue, clock)
	require.NoError(t, dao.PopulateReminderQueue(ctx))
	require.True(t, util.IsChannelEmpty(newQueue))

	err = client.Disconnect(ctx)
	if err != nil {
		panic(err)
	}

	require.NotNil(t, dao.PopulateReminderQueue(ctx))
}
