package reminder

import (
	"context"
	"log"
	"time"

	"yandexschooldating/clock"

	"github.com/joomcode/errorx"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type Reminder struct {
	UnixTime int64  `bson:"unixTime"`
	ChatID   int64  `bson:"chatId"`
	Text     string `bson:"text"`
}

//goland:noinspection GoNameStartsWithPackageName
var ReminderBSON = struct {
	UnixTime string
	ChatID   string
	Text     string
}{"unixTime", "chatId", "text"}

type DAO interface {
	AddReminder(ctx context.Context, reminderTime time.Time, chatID int64, text string) error
	PopulateReminderQueue(ctx context.Context) error
}

type dao struct {
	reminders *mongo.Collection
	queue     chan<- Reminder
	clock     clock.Clock
}

var _ DAO = (*dao)(nil)

func NewDAO(client *mongo.Client, database string, queue chan<- Reminder, clock clock.Clock) DAO {
	return &dao{
		reminders: client.Database(database).Collection("reminders"),
		queue:     queue,
		clock:     clock,
	}
}

func (m *dao) AddReminder(ctx context.Context, reminderTime time.Time, chatID int64, text string) error {
	reminder := Reminder{
		UnixTime: reminderTime.Unix(),
		ChatID:   chatID,
		Text:     text,
	}
	currentTime := m.clock.Now().Unix()
	if currentTime > reminder.UnixTime {
		return errorx.IllegalState.New("reminders must be in the future")
	}
	timer := time.NewTimer(time.Duration(reminder.UnixTime-currentTime) * time.Second)
	go func() {
		<-timer.C
		m.queue <- reminder
	}()
	log.Printf("saving reminder %+v", reminder)
	_, err := m.reminders.InsertOne(ctx, reminder)
	return err
}

func (m *dao) PopulateReminderQueue(ctx context.Context) error {
	currentTime := m.clock.Now().Unix()
	cursor, err := m.reminders.Find(ctx, bson.M{ReminderBSON.UnixTime: bson.M{"$gt": currentTime}})
	if err != nil {
		return err
	}

	for cursor.Next(ctx) {
		var reminder Reminder
		err = cursor.Decode(&reminder)
		log.Printf("restoring reminder %+v", reminder)
		if err != nil {
			return errorx.Decorate(err, "can't decode reminder")
		}
		seconds := reminder.UnixTime - currentTime
		if seconds <= 0 {
			return errorx.IllegalState.New("requested timer duration < 0")
		}
		duration := time.Duration(seconds) * time.Second
		timer := time.NewTimer(duration)
		go func() {
			<-timer.C
			m.queue <- reminder
		}()
	}
	return nil
}
