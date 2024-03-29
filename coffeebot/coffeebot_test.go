package coffeebot_test

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"yandexschooldating/clock"
	"yandexschooldating/coffeebot"
	"yandexschooldating/config"
	"yandexschooldating/match"
	"yandexschooldating/messagestrings"
	"yandexschooldating/reminder"
	"yandexschooldating/user"
	"yandexschooldating/util"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func requireSingleReplyText(t *testing.T, replies []coffeebot.BotReply, expectedChatID int64, expectedText string) {
	require.Len(t, replies, 1)
	require.Equal(t, expectedChatID, replies[0].ChatID)
	require.Equal(t, expectedText, replies[0].Text)
}

type fakeMatchDAO struct {
	addMatchCalls int
	matchingCycle int
}

func (f *fakeMatchDAO) BreakMatchForUser(context.Context, int) error {
	panic("unimplemented")
}

func (f *fakeMatchDAO) GetAllMatchedUsers(context.Context) ([]int, error) {
	panic("unimplemented")
}

func (f *fakeMatchDAO) FindCurrentMatchForUserID(context.Context, int) (*match.Match, error) {
	panic("unimplemented")
}

func (f *fakeMatchDAO) AddMatch(context.Context, int, int) error {
	f.addMatchCalls++
	return nil
}

func (f *fakeMatchDAO) UpdateMatchTime(context.Context, int, time.Time) error {
	panic("unimplemented")
}

func (f *fakeMatchDAO) IncrementMatchingCycle() {
	f.matchingCycle++
}

func randSeq() string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyz")
	b := make([]rune, 6)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

type testContext struct {
	database    string
	client      *mongo.Client
	userDAO     *user.DAO
	clock       clock.Clock
	matchDAO    coffeebot.MatchDAO
	queue       chan reminder.Reminder
	reminderDAO *reminder.DAO
	bot         *coffeebot.CoffeeBot

	removeMarkup                         int
	citiesKeyboard                       int
	remindStopMeetingsKeyboard           int
	remindChangeTimeStopMeetingsKeyboard int
	activateKeyboard                     int
}

func newTestContext(ctx context.Context) testContext {
	testDatabase := "test_coffeebot" + randSeq()
	client, err := util.GetMongoClient(ctx, config.MongoUri, 2*time.Second)
	if err != nil {
		panic(err)
	}
	util.DropTestDatabaseOrPanic(ctx, client, testDatabase)
	return testContext{database: testDatabase, client: client}
}

func (m *testContext) init(ctx context.Context, clock clock.Clock) func() {
	m.removeMarkup = 1
	m.citiesKeyboard = 2
	m.remindStopMeetingsKeyboard = 3
	m.remindChangeTimeStopMeetingsKeyboard = 4
	m.activateKeyboard = 5

	m.userDAO = user.NewDAO(m.client, m.database)

	m.clock = clock
	m.matchDAO = match.NewDAO(m.client, m.database, m.clock)
	err := m.matchDAO.(*match.DAO).InitializeMatchingCycle(ctx)
	if err != nil {
		panic(err)
	}
	m.queue = make(chan reminder.Reminder)
	m.reminderDAO = reminder.NewDAO(m.client, m.database, m.queue, m.clock)
	err = m.reminderDAO.PopulateReminderQueue(ctx)
	if err != nil {
		panic(err)
	}
	m.bot = coffeebot.NewCoffeeBot(
		m.userDAO,
		m.matchDAO,
		m.reminderDAO,
		m.clock,
		&m.removeMarkup,
		&m.citiesKeyboard,
		&m.remindStopMeetingsKeyboard,
		&m.remindChangeTimeStopMeetingsKeyboard,
		&m.activateKeyboard,
	)
	return func() { util.DropTestDatabaseOrPanic(ctx, m.client, m.database) }
}

func TestCoffeeBot(t *testing.T) {
	ctx := context.Background()

	t.Run("Chat test with 4 London users, 4 Moscow users and 2 users from obscure places", func(t *testing.T) {
		fakeClock := clock.Fake{Current: time.Date(2020, 7, 5, 4, 20, 0, 0, time.UTC)}
		test := newTestContext(ctx)
		defer test.init(ctx, &fakeClock)()

		err := test.bot.MakeMatches(ctx, fakeClock.Now().Add(3*time.Second))
		require.NoError(t, err)

		replies, err := test.bot.ProcessMessage(ctx, 555, "", 555, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 555, messagestrings.SorryNoUsername)

		replies, err = test.bot.ProcessMessage(ctx, 66, "", 66, "Привет!")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 66, messagestrings.SorryNoUsername)

		_, err = test.bot.ProcessMessage(ctx, 1, "john", 1, messagestrings.RemindMe)
		require.Error(t, err)

		replies, err = test.bot.ProcessMessage(ctx, 1, "john", 1, "Привет!")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.DefaultReply)

		replies, err = test.bot.ProcessMessage(ctx, 1, "john", 1, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.GreetingAskCity)

		replies, err = test.bot.ProcessMessage(ctx, 1, "john", 1, "Москва")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 1, "john", 1, "ехехе")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.DefaultReply)

		replies, err = test.bot.ProcessMessage(ctx, 1, "john", 1, messagestrings.RemindMe)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.NoMeetingsThisWeek)

		err = test.bot.MakeMatches(ctx, fakeClock.Now().Add(3*time.Second))
		require.NoError(t, err)

		start := time.Now()
		tick := <-test.queue
		elapsed := time.Since(start).Seconds()
		fakeClock.Current = fakeClock.Current.Add(10 * time.Second)
		require.True(t, util.IsChannelEmpty(test.queue))
		require.LessOrEqual(t, 2.0, elapsed)
		require.LessOrEqual(t, elapsed, 4.0)
		require.Equal(t, int64(1), tick.ChatID)
		require.Equal(t, messagestrings.CouldNotFindMatch, tick.Text)

		require.Equal(t, mongo.ErrNoDocuments, test.client.Database(test.database).Collection("matches").FindOne(ctx, bson.M{}).Err())

		replies, err = test.bot.ProcessMessage(ctx, 2, "jack", 2, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 2, "jack", 2, "Москва")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 3, "fedor", 3, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 3, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 3, "fedor", 3, "Москва")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 3, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 4, "alex", 4, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 4, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 4, "alex", 4, "Москва")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 4, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 5, "tema", 5, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 5, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 5, "tema", 5, "Лондон")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 5, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 6, "anya", 6, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 6, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 6, "anya", 6, "Лондон")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 6, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 7, "alisa", 7, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 7, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 7, "alisa", 7, "Лондон")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 7, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 8, "danila", 8, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 8, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 8, "danila", 8, "Лондон")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 8, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 9, "druzhko", 9, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 9, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 9, "druzhko", 9, "Шахты")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 9, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 10, "msch", 10, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 10, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 10, "msch", 10, "Рыбинск")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 10, messagestrings.Welcome)

		err = test.bot.MakeMatches(ctx, fakeClock.Now().Add(3*time.Second))
		require.NoError(t, err)

		var reminders []reminder.Reminder
		start = time.Now()

		for i := 0; i < 10; i++ {
			reminders = append(reminders, <-test.queue)
		}

		elapsed = time.Since(start).Seconds()
		fakeClock.Current = fakeClock.Current.Add(10 * time.Second)
		require.True(t, util.IsChannelEmpty(test.queue))
		require.LessOrEqual(t, 2.0, elapsed)
		require.LessOrEqual(t, elapsed, 4.0)
		for _, i := range reminders {
			require.NotEqual(t, messagestrings.CouldNotFindMatch, i.Text)
		}

		count, err := test.client.Database(test.database).Collection("matches").CountDocuments(ctx, bson.M{})
		if err != nil {
			panic(err)
		}
		require.Equal(t, int64(5), count)

		moscow := map[int]bool{1: true, 2: true, 3: true, 4: true}
		london := map[int]bool{5: true, 6: true, 7: true, 8: true}

		checkMatches := func() {
			cursor, err := test.client.Database(test.database).Collection("matches").Find(ctx, bson.M{})
			if err != nil {
				panic(err)
			}
			for cursor.Next(ctx) {
				var m match.Match
				err = cursor.Decode(&m)
				if err != nil {
					panic(err)
				}
				fmt.Printf("%+v\n", m)
				require.Equal(t, moscow[m.FirstID], moscow[m.SecondID])
				require.Equal(t, london[m.FirstID], london[m.SecondID])
				require.Equal(t, m.FirstID == 9, m.SecondID == 10)
			}
		}
		checkMatches()

		fakeClock.Current = fakeClock.Current.Add(1 * time.Second)
		test.init(ctx, &fakeClock)
		require.True(t, util.IsChannelEmpty(test.queue))

		err = test.bot.MakeMatches(ctx, fakeClock.Now().Add(5*time.Second))
		require.NoError(t, err)

		start = time.Now()

		test.init(ctx, &fakeClock)

		for i := 0; i < 10; i++ {
			reminders = append(reminders, <-test.queue)
		}

		elapsed = time.Since(start).Seconds()
		fakeClock.Current = fakeClock.Current.Add(5 * time.Second)
		require.LessOrEqual(t, 3.0, elapsed)
		require.LessOrEqual(t, elapsed, 7.0)
		require.True(t, util.IsChannelEmpty(test.queue))
		for _, i := range reminders {
			require.NotEqual(t, messagestrings.CouldNotFindMatch, i.Text)
		}

		count, err = test.client.Database(test.database).Collection("matches").CountDocuments(ctx, bson.M{})
		if err != nil {
			panic(err)
		}
		require.Equal(t, int64(10), count)
		checkMatches()

		replies, err = test.bot.ProcessMessage(ctx, 9, "druzhko", 9, messagestrings.RemindMe)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 9, "У тебя встреча с @msch. Чтобы получить сообщение перед встречей, напиши время встречи в формате число.месяц часы:минуты, например 02.01 15:04. Поскольку мы не знаем часового пояса для твоего города, время должно быть в формате UTC")

		replies, err = test.bot.ProcessMessage(ctx, 9, "druzhko", 9, "ОО:ОО АА.АА")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 9, messagestrings.CouldNotParseTime)

		replies, err = test.bot.ProcessMessage(ctx, 9, "druzhko", 9, messagestrings.RemindMe)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 9, "У тебя встреча с @msch. Чтобы получить сообщение перед встречей, напиши время встречи в формате число.месяц часы:минуты, например 02.01 15:04. Поскольку мы не знаем часового пояса для твоего города, время должно быть в формате UTC")

		replies, err = test.bot.ProcessMessage(ctx, 9, "druzhko", 9, "05.07 6:00")
		require.NoError(t, err)
		require.Len(t, replies, 2)
		require.NotEqual(t, messagestrings.CouldNotFindMatch, replies[0].Text)
		require.NotEqual(t, messagestrings.CouldNotFindMatch, replies[1].Text)
		if replies[0].ChatID == 9 {
			require.Equal(t, "Встречи в твоём городе не нашлось. Встреча с @msch будет 05 July в 06:00 UTC", replies[0].Text)
			require.Equal(t, "Встречи в твоём городе не нашлось. Встреча с @druzhko будет 05 July в 06:00 UTC", replies[1].Text)
		} else {
			require.Equal(t, "Встречи в твоём городе не нашлось. Встреча с @druzhko будет 05 July в 06:00 UTC", replies[0].Text)
			require.Equal(t, "Встречи в твоём городе не нашлось. Встреча с @msch будет 05 July в 06:00 UTC", replies[1].Text)
		}

		_, err = test.bot.ProcessMessage(ctx, 1, "john", 1, messagestrings.RemindMe)
		require.NoError(t, err)

		replies, err = test.bot.ProcessMessage(ctx, 1, "john", 1, "04.07 6:00")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.TimeInThePast)
	})

	t.Run("Same city reminders", func(t *testing.T) {
		fakeClock := clock.Fake{Current: time.Date(2020, 7, 5, 5, 59, 56, 0, time.UTC)}
		test := newTestContext(ctx)
		defer test.init(ctx, &fakeClock)()

		replies, err := test.bot.ProcessMessage(ctx, 1, "vikki", 1, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 1, "vikki", 1, "Минск")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, "Минск")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.Welcome)

		err = test.bot.MakeMatches(ctx, fakeClock.Now().Add(1*time.Second))
		require.NoError(t, err)
		start := time.Now()
		<-test.queue
		<-test.queue
		elapsed := time.Since(start).Seconds()
		fakeClock.Current = fakeClock.Current.Add(1 * time.Second)
		require.True(t, util.IsChannelEmpty(test.queue))
		require.LessOrEqual(t, elapsed, 2.0)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, messagestrings.RemindMe)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, "У тебя встреча с @vikki. Чтобы получить сообщение перед встречей, напиши время встречи в формате число.месяц часы:минуты, например 02.01 15:04")

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, "05.07 9:00")
		require.NoError(t, err)
		require.Len(t, replies, 2)
		if replies[0].ChatID == 1 {
			require.Equal(t, "Встреча с @vance будет 05 July в 09:00 +03", replies[0].Text)
			require.Equal(t, "Встреча с @vikki будет 05 July в 09:00 +03", replies[1].Text)
		} else {
			require.Equal(t, "Встреча с @vikki будет 05 July в 09:00 +03", replies[0].Text)
			require.Equal(t, "Встреча с @vance будет 05 July в 09:00 +03", replies[1].Text)
		}

		start = time.Now()

		replies, err = test.bot.ProcessMessage(ctx, 1, "vikki", 1, messagestrings.RemindMe)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, "Встреча с @vance будет 05 July в 09:00 +03")
		require.Equal(t, &test.remindChangeTimeStopMeetingsKeyboard, replies[0].Markup)

		tick1 := <-test.queue
		tick2 := <-test.queue
		elapsed = time.Since(start).Seconds()
		require.True(t, util.IsChannelEmpty(test.queue))
		require.LessOrEqual(t, 2.0, elapsed)
		require.LessOrEqual(t, elapsed, 4.0)
		require.True(t, (tick1.Text == "Встреча с @vikki будет 05 July в 09:00 +03" && tick2.Text == "Встреча с @vance будет 05 July в 09:00 +03") || (tick2.Text == "Встреча с @vikki будет 05 July в 09:00 +03" && tick1.Text == "Встреча с @vance будет 05 July в 09:00 +03"))
	})

	t.Run("Different city timezone formatting", func(t *testing.T) {
		fakeClock := clock.Fake{Current: time.Date(2020, 11, 5, 4, 20, 0, 0, time.UTC)}
		test := newTestContext(ctx)
		defer test.init(ctx, &fakeClock)()

		replies, err := test.bot.ProcessMessage(ctx, 1, "riazanovskiy", 1, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 1, "riazanovskiy", 1, "Лондон")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 2, "sasha", 2, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 2, "sasha", 2, "Москва")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.Welcome)

		err = test.bot.MakeMatches(ctx, fakeClock.Now().Add(1*time.Second))
		require.NoError(t, err)
		start := time.Now()
		<-test.queue
		<-test.queue
		elapsed := time.Since(start).Seconds()
		require.True(t, util.IsChannelEmpty(test.queue))
		require.LessOrEqual(t, elapsed, 2.0)

		replies, err = test.bot.ProcessMessage(ctx, 2, "sasha", 2, messagestrings.RemindMe)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, "У тебя встреча с @riazanovskiy. Чтобы получить сообщение перед встречей, напиши время встречи в формате число.месяц часы:минуты, например 02.01 15:04")

		replies, err = test.bot.ProcessMessage(ctx, 2, "sasha", 2, "07.11 9:00")
		require.NoError(t, err)
		require.Len(t, replies, 2)
		if replies[0].ChatID == 1 {
			require.Equal(t, "Встречи в твоём городе не нашлось. Встреча с @sasha будет 07 November в 06:00 GMT", replies[0].Text)
			require.Equal(t, "Встречи в твоём городе не нашлось. Встреча с @riazanovskiy будет 07 November в 09:00 MSK", replies[1].Text)
		} else {
			require.Equal(t, "Встречи в твоём городе не нашлось. Встреча с @riazanovskiy будет 07 November в 09:00 MSK", replies[0].Text)
			require.Equal(t, "Встречи в твоём городе не нашлось. Встреча с @sasha будет 07 November в 06:00 GMT", replies[1].Text)
		}

		replies, err = test.bot.ProcessMessage(ctx, 1, "riazanovskiy", 1, messagestrings.RemindMe)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, "Встречи в твоём городе не нашлось. Встреча с @sasha будет 07 November в 06:00 GMT")
		require.Equal(t, &test.remindChangeTimeStopMeetingsKeyboard, replies[0].Markup)
	})

	t.Run("Broken client", func(t *testing.T) {
		// yes this is remarkably stupid
		fakeClock := clock.Fake{Current: time.Date(2020, 7, 5, 4, 20, 0, 0, time.UTC)}
		test := newTestContext(ctx)
		test.init(ctx, &fakeClock)

		err := test.client.Disconnect(ctx)
		if err != nil {
			panic(err)
		}

		_, err = test.bot.ProcessMessage(ctx, 1, "john", 1, messagestrings.RemindMe)
		require.Error(t, err)
		err = test.bot.MakeMatches(ctx, fakeClock.Now().Add(3*time.Second))
		require.Error(t, err)

		test = newTestContext(ctx)
		test.init(ctx, &fakeClock)
		replies, err := test.bot.ProcessMessage(ctx, 1, "john", 1, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.GreetingAskCity)
		err = test.client.Disconnect(ctx)
		if err != nil {
			panic(err)
		}
		_, err = test.bot.ProcessMessage(ctx, 1, "john", 1, "Москва")
		require.Error(t, err)

		_, err = test.bot.ProcessMessage(ctx, 1, "john", 1, messagestrings.Activate)
		require.Error(t, err)

		test = newTestContext(ctx)
		test.database = "test_coffeebot"
		test.init(ctx, &fakeClock)

		replies, err = test.bot.ProcessMessage(ctx, 1, "john", 1, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.GreetingAskCity)

		replies, err = test.bot.ProcessMessage(ctx, 1, "john", 1, "Москва")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 2, "jack", 2, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 2, "jack", 2, "Лондон")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.Welcome)

		err = test.bot.MakeMatches(ctx, fakeClock.Now().Add(3*time.Second))
		require.NoError(t, err)

		replies, err = test.bot.ProcessMessage(ctx, 2, "jack", 2, messagestrings.RemindMe)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, "У тебя встреча с @john. Чтобы получить сообщение перед встречей, напиши время встречи в формате число.месяц часы:минуты, например 02.01 15:04")

		err = test.client.Disconnect(ctx)
		if err != nil {
			panic(err)
		}

		_, err = test.bot.ProcessMessage(ctx, 2, "jack", 2, "05.07 9:15")
		require.Error(t, err)

		replies, err = test.bot.ProcessMessage(ctx, 2128506, config.AdminUser, 2128506, "MakeMatches")
		require.NoError(t, err)
		require.Len(t, replies, 1)
		require.True(t, strings.HasPrefix(replies[0].Text, "MakeMatches error: "))
	})

	t.Run("MakeMatches", func(t *testing.T) {
		fakeClock := clock.Fake{Current: time.Date(2020, 7, 5, 4, 20, 0, 0, time.UTC)}
		test := newTestContext(ctx)
		defer test.init(ctx, &fakeClock)()

		fakeMatches := fakeMatchDAO{0, 0}
		test.bot = coffeebot.NewCoffeeBot(
			test.userDAO,
			&fakeMatches,
			test.reminderDAO,
			&fakeClock,
			&test.removeMarkup,
			&test.citiesKeyboard,
			&test.remindStopMeetingsKeyboard,
			&test.remindChangeTimeStopMeetingsKeyboard,
			&test.activateKeyboard,
		)

		replies, err := test.bot.ProcessMessage(ctx, 9, "druzhko", 9, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 9, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 9, "druzhko", 9, "Шахты")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 9, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 10, "msch", 10, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 10, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 10, "msch", 10, "Рыбинск")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 10, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 1, "john", 1, "MakeMatches")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.DefaultReply)

		replies, err = test.bot.ProcessMessage(ctx, 2128506, config.AdminUser, 2128506, "MakeMatches")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2128506, "MakeMatches succeeded")
		require.Equal(t, 1, fakeMatches.addMatchCalls)
		require.Equal(t, 1, fakeMatches.matchingCycle)
	})

	t.Run("Lost meeting", func(t *testing.T) {
		fakeClock := clock.Fake{Current: time.Date(2020, 7, 5, 4, 20, 0, 0, time.UTC)}
		test := newTestContext(ctx)
		defer test.init(ctx, &fakeClock)()

		replies, err := test.bot.ProcessMessage(ctx, 1, "john", 1, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.GreetingAskCity)

		replies, err = test.bot.ProcessMessage(ctx, 1, "john", 1, "Москва")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 2, "jack", 2, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 2, "jack", 2, "Лондон")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.Welcome)

		err = test.bot.MakeMatches(ctx, fakeClock.Now().Add(3*time.Second))
		require.NoError(t, err)

		replies, err = test.bot.ProcessMessage(ctx, 2, "jack", 2, messagestrings.RemindMe)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, "У тебя встреча с @john. Чтобы получить сообщение перед встречей, напиши время встречи в формате число.месяц часы:минуты, например 02.01 15:04")

		replies, err = test.bot.ProcessMessage(ctx, 3, "fedor", 3, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 3, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 3, "fedor", 3, "Москва")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 3, messagestrings.Welcome)

		err = test.bot.MakeMatches(ctx, fakeClock.Now().Add(3*time.Second))
		require.NoError(t, err)

		replies, err = test.bot.ProcessMessage(ctx, 2, "jack", 2, "05.07 9:15")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.NoMeetingsThisWeek)
	})

	t.Run("Stop meetings", func(t *testing.T) {
		fakeClock := clock.Fake{Current: time.Date(2020, 7, 5, 5, 59, 56, 0, time.UTC)}
		test := newTestContext(ctx)
		defer test.init(ctx, &fakeClock)()

		replies, err := test.bot.ProcessMessage(ctx, 1, "vikki", 1, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 1, "vikki", 1, "Минск")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, "Минск")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 1, "vikki", 1, messagestrings.StopMeetings)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.InactiveUser)

		replies, err = test.bot.ProcessMessage(ctx, 1, "vikki", 1, messagestrings.StopMeetings)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.InactiveUser)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, messagestrings.StopMeetings)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.InactiveUser)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, messagestrings.StopMeetings)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.InactiveUser)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, messagestrings.RemindMe)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.InactiveUser)

		fakeMatches := fakeMatchDAO{0, 0}
		test.bot = coffeebot.NewCoffeeBot(
			test.userDAO,
			&fakeMatches,
			test.reminderDAO,
			&fakeClock,
			&test.removeMarkup,
			&test.citiesKeyboard,
			&test.remindStopMeetingsKeyboard,
			&test.remindChangeTimeStopMeetingsKeyboard,
			&test.activateKeyboard,
		)
		err = test.bot.MakeMatches(ctx, fakeClock.Now().Add(1*time.Second))
		require.NoError(t, err)
		require.Equal(t, 0, fakeMatches.addMatchCalls)
	})

	t.Run("Stop meetings with match", func(t *testing.T) {
		fakeClock := clock.Fake{Current: time.Date(2020, 7, 5, 5, 59, 56, 0, time.UTC)}
		test := newTestContext(ctx)
		defer test.init(ctx, &fakeClock)()

		replies, err := test.bot.ProcessMessage(ctx, 1, "vikki", 1, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 1, "vikki", 1, "Минск")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, "Минск")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.Welcome)

		err = test.bot.MakeMatches(ctx, fakeClock.Now().Add(1*time.Second))
		require.NoError(t, err)
		start := time.Now()
		<-test.queue
		<-test.queue
		elapsed := time.Since(start).Seconds()
		fakeClock.Current = fakeClock.Current.Add(1 * time.Second)
		require.True(t, util.IsChannelEmpty(test.queue))
		require.LessOrEqual(t, elapsed, 2.0)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, messagestrings.RemindMe)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, "У тебя встреча с @vikki. Чтобы получить сообщение перед встречей, напиши время встречи в формате число.месяц часы:минуты, например 02.01 15:04")

		replies, err = test.bot.ProcessMessage(ctx, 1, "vikki", 1, messagestrings.StopMeetings)
		require.NoError(t, err)
		require.Len(t, replies, 2)
		if replies[0].ChatID == 2 {
			replies[1], replies[0] = replies[0], replies[1]
		}
		require.Equal(t, int64(1), replies[0].ChatID)
		require.Equal(t, messagestrings.InactiveUser, replies[0].Text)
		require.Equal(t, int64(2), replies[1].ChatID)
		require.Equal(t, messagestrings.PartnerRefused, replies[1].Text)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, "05.07 9:00")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.NoMeetingsThisWeek)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, messagestrings.StopMeetings)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.InactiveUser)
	})

	t.Run("Stop meetings with replacement", func(t *testing.T) {
		fakeClock := clock.Fake{Current: time.Date(2020, 7, 5, 5, 59, 56, 0, time.UTC)}
		test := newTestContext(ctx)
		defer test.init(ctx, &fakeClock)()

		replies, err := test.bot.ProcessMessage(ctx, 1, "vikki", 1, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 1, "vikki", 1, "Минск")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, "Минск")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 3, "nancy", 3, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 3, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 3, "nancy", 3, "Лондон")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 3, messagestrings.Welcome)

		err = test.bot.MakeMatches(ctx, fakeClock.Now().Add(1*time.Second))
		require.NoError(t, err)
		start := time.Now()
		<-test.queue
		<-test.queue
		<-test.queue
		elapsed := time.Since(start).Seconds()
		fakeClock.Current = fakeClock.Current.Add(1 * time.Second)
		require.True(t, util.IsChannelEmpty(test.queue))
		require.LessOrEqual(t, elapsed, 2.0)

		replies, err = test.bot.ProcessMessage(ctx, 3, "nancy", 3, messagestrings.RemindMe)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 3, messagestrings.NoMeetingsThisWeek)

		replies, err = test.bot.ProcessMessage(ctx, 1, "vikki", 1, messagestrings.StopMeetings)
		require.NoError(t, err)
		require.Len(t, replies, 4)

		replies, err = test.bot.ProcessMessage(ctx, 1, "vikki", 1, "лапки")
		require.NoError(t, err)
		require.Len(t, replies, 1)
		require.Equal(t, &test.activateKeyboard, replies[0].Markup)

		replies, err = test.bot.ProcessMessage(ctx, 3, "nancy", 3, messagestrings.RemindMe)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 3, "У тебя встреча с @vance. Чтобы получить сообщение перед встречей, напиши время встречи в формате число.месяц часы:минуты, например 02.01 15:04")

		replies, err = test.bot.ProcessMessage(ctx, 3, "nancy", 3, "aaaaa")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 3, messagestrings.CouldNotParseTime)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, messagestrings.RemindMe)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, "У тебя встреча с @nancy. Чтобы получить сообщение перед встречей, напиши время встречи в формате число.месяц часы:минуты, например 02.01 15:04")
	})

	t.Run("Activate", func(t *testing.T) {
		fakeClock := clock.Fake{Current: time.Date(2020, 7, 5, 4, 20, 0, 0, time.UTC)}
		test := newTestContext(ctx)
		defer test.init(ctx, &fakeClock)()

		replies, err := test.bot.ProcessMessage(ctx, 1, "vikki", 1, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 1, "vikki", 1, "Минск")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, "Минск")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 1, "vikki", 1, messagestrings.StopMeetings)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.InactiveUser)

		replies, err = test.bot.ProcessMessage(ctx, 1, "vikki", 1, messagestrings.StopMeetings)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.InactiveUser)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, messagestrings.StopMeetings)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.InactiveUser)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, messagestrings.StopMeetings)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.InactiveUser)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, messagestrings.RemindMe)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.InactiveUser)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, messagestrings.Activate)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.NowActive)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, messagestrings.Activate)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.AlreadyActive)

		replies, err = test.bot.ProcessMessage(ctx, 1, "vikki", 1, messagestrings.Activate)
		require.NoError(t, err)
		require.Len(t, replies, 3)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, messagestrings.RemindMe)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, "У тебя встреча с @vikki. Чтобы получить сообщение перед встречей, напиши время встречи в формате число.месяц часы:минуты, например 02.01 15:04")

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, "05.07 7:30")
		require.NoError(t, err)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, messagestrings.StopMeetings)
		require.NoError(t, err)
		require.Len(t, replies, 2)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, messagestrings.Activate)
		require.NoError(t, err)

		err = test.bot.MakeMatches(ctx, fakeClock.Now().Add(1*time.Second))
		require.NoError(t, err)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, messagestrings.RemindMe)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, "У тебя встреча с @vikki. Чтобы получить сообщение перед встречей, напиши время встречи в формате число.месяц часы:минуты, например 02.01 15:04")

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, "05.07 5:00")
		require.NoError(t, err)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, messagestrings.StopMeetings)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.InactiveUser)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, messagestrings.Activate)
		require.NoError(t, err)
		require.Len(t, replies, 3)

		// Yes, we assume that new users won't get a match until next Monday
		// Potentially this can be changed
		replies, err = test.bot.ProcessMessage(ctx, 3, "nancy", 3, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 3, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 3, "nancy", 3, "Лондон")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 3, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 3, "nancy", 3, messagestrings.RemindMe)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 3, messagestrings.NoMeetingsThisWeek)

		err = test.bot.MakeMatches(ctx, fakeClock.Now().Add(1*time.Second))
		require.NoError(t, err)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, messagestrings.RemindMe)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, "У тебя встреча с @vikki. Чтобы получить сообщение перед встречей, напиши время встречи в формате число.месяц часы:минуты, например 02.01 15:04")

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, messagestrings.StopMeetings)
		require.NoError(t, err)
		require.Len(t, replies, 4)

		replies, err = test.bot.ProcessMessage(ctx, 2, "vance", 2, messagestrings.Activate)
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.NowActive)
	})

	t.Run("Remote-first matches", func(t *testing.T) {
		// this test can fail spuriously
		// I am truly sorry
		fakeClock := clock.Fake{Current: time.Date(2020, 7, 5, 4, 20, 0, 0, time.UTC)}
		test := newTestContext(ctx)
		defer test.init(ctx, &fakeClock)()

		replies, err := test.bot.ProcessMessage(ctx, 1, "john", 1, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.GreetingAskCity)

		replies, err = test.bot.ProcessMessage(ctx, 1, "john", 1, "Москва")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 1, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 2, "jack", 2, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 2, "jack", 2, "Москва")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 2, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 5, "tema", 5, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 5, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 5, "tema", 5, "Лондон")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 5, messagestrings.Welcome)

		replies, err = test.bot.ProcessMessage(ctx, 6, "anya", 6, "/start")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 6, messagestrings.GreetingAskCity)
		replies, err = test.bot.ProcessMessage(ctx, 6, "anya", 6, "Лондон")
		require.NoError(t, err)
		requireSingleReplyText(t, replies, 6, messagestrings.Welcome)

		updateResult, err := test.client.Database(test.database).Collection("users").UpdateMany(ctx, bson.M{}, bson.M{"$set": bson.M{user.UserBSON.RemoteFirst: true}})
		require.NoError(t, err)
		require.Equal(t, int64(4), updateResult.ModifiedCount)

		ok := false
		for i := 0; i < 10; i++ {
			err = test.bot.MakeMatches(ctx, fakeClock.Now().Add(3*time.Second))
			require.NoError(t, err)
			fakeClock.Current = fakeClock.Current.Add(10 * time.Second)

			m, err := test.matchDAO.FindCurrentMatchForUserID(ctx, 1)
			require.NoError(t, err)
			require.NotNil(t, m)

			if m.SecondID != 2 {
				ok = true
				break
			}
		}

		require.True(t, ok)
	})
}
