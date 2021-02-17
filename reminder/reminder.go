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

type DAO struct {
	reminders *mongo.Collection
	queue     chan<- Reminder
	clock     clock.Clock
}

func NewDAO(client *mongo.Client, database string, queue chan<- Reminder, clock clock.Clock) *DAO {
	return &DAO{
		reminders: client.Database(database).Collection("reminders"),
		queue:     queue,
		clock:     clock,
	}
}

func (m *DAO) AddReminder(ctx context.Context, reminderTime time.Time, chatID int64, text string) error {
	reminder := Reminder{
		UnixTime: reminderTime.Unix(),
		ChatID:   chatID,
		Text:     text,
	}
	seconds := reminder.UnixTime - m.clock.Now().Unix()
	if seconds < 0 {
		return errorx.IllegalState.New("reminders must be in the future")
	}
	m.startTimer(reminder, seconds)
	log.Printf("saving reminder %+v", reminder)
	_, err := m.reminders.InsertOne(ctx, reminder)
	return err
}

func (m *DAO) PopulateReminderQueue(ctx context.Context) error {
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
			log.Printf("requested timer duration < 0, reminder %+v is lost", reminder)
			continue
		}
		m.startTimer(reminder, seconds)
	}
	return nil
}

func (m *DAO) startTimer(reminder Reminder, seconds int64) {
	time.AfterFunc(time.Duration(seconds)*time.Second, func() { m.queue <- reminder })
}
