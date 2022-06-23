package main

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"yandexschooldating/clock"
	"yandexschooldating/coffeebot"
	"yandexschooldating/config"
	"yandexschooldating/match"
	"yandexschooldating/messagestrings"
	"yandexschooldating/reminder"
	"yandexschooldating/user"
	"yandexschooldating/util"

	"github.com/davecgh/go-spew/spew"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

var DomesticKeyboard = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton(messagestrings.Moscow),
		tgbotapi.NewKeyboardButton(messagestrings.StPetersburg),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton(messagestrings.Minsk),
		tgbotapi.NewKeyboardButton(messagestrings.Novosibirsk),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton(messagestrings.Yekaterinburg),
		tgbotapi.NewKeyboardButton(messagestrings.NizhnyNovgorod),
	),
)

var WorldKeyboard = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton(messagestrings.Moscow),
		tgbotapi.NewKeyboardButton(messagestrings.Minsk),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton(messagestrings.TelAviv),
		tgbotapi.NewKeyboardButton(messagestrings.Yerevan),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton(messagestrings.NewYork),
		tgbotapi.NewKeyboardButton(messagestrings.Tbilisi),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton(messagestrings.London),
		tgbotapi.NewKeyboardButton(messagestrings.Berlin),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton(messagestrings.Zurich),
		tgbotapi.NewKeyboardButton(messagestrings.Istanbul),
	),
)

func main() {
	spew.Config.Indent = ""

	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s bot_token.txt", os.Args[0])
	}
	tokenPath := os.Args[1]
	token, err := ioutil.ReadFile(tokenPath)
	if err != nil {
		log.Fatalf("secret token not available %+v", err)
	}

	bot, err := tgbotapi.NewBotAPI(strings.TrimSpace(string(token)))
	if err != nil {
		log.Fatal(err)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	client, err := util.GetMongoClient(ctx, config.MongoUri, config.MongoTimeout)
	if err != nil {
		log.Panic(err)
	}

	userDAO := user.NewDAO(client, config.Database)
	realClock := clock.NewRealClock()
	matchDAO := match.NewDAO(client, config.Database, realClock)
	err = matchDAO.InitializeMatchingCycle(ctx)
	if err != nil {
		log.Panic(err)
	}

	remindersChan := make(chan reminder.Reminder)
	matchTimerChan := InitMatchTimerChan(realClock)

	remindersDAO := reminder.NewDAO(client, config.Database, remindersChan, realClock)

	err = remindersDAO.PopulateReminderQueue(ctx)
	if err != nil {
		log.Panicf("can't restore old timers %+v", err)
	}

	removeMarkup := tgbotapi.NewRemoveKeyboard(true)

	citiesKeyboard := WorldKeyboard

	remindStopMeetingsKeyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(messagestrings.RemindMe),
			tgbotapi.NewKeyboardButton(messagestrings.StopMeetings),
		),
	)

	remindChangeTimeStopMeetingsKeyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(messagestrings.RemindMe),
			tgbotapi.NewKeyboardButton(messagestrings.ChangeTime),
			tgbotapi.NewKeyboardButton(messagestrings.StopMeetings),
		),
	)

	activateKeyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(messagestrings.Activate),
		),
	)

	coffeeBot := coffeebot.NewCoffeeBot(
		userDAO,
		matchDAO,
		remindersDAO,
		realClock,
		removeMarkup,
		citiesKeyboard,
		remindStopMeetingsKeyboard,
		remindChangeTimeStopMeetingsKeyboard,
		activateKeyboard,
	)

	for {
		select {
		case update := <-updates:
			if update.Message == nil {
				continue
			}

			log.Printf("[%s] [%d] [%d] %s %+v", update.Message.From.UserName, update.Message.From.ID, update.Message.Date, update.Message.Text, strings.Replace(spew.Sdump(update), "\n", " ", -1))
			replies, err := coffeeBot.ProcessMessage(ctx, update.Message.From.ID, update.Message.From.UserName, update.Message.Chat.ID, update.Message.Text)
			if err != nil {
				log.Printf("can't get reply %+v", err)
				replies = []coffeebot.BotReply{{ChatID: update.Message.Chat.ID, Text: "Произошла ужасная ошибка, напиши @" + config.AdminUser, Markup: nil}}
			}
			for i, reply := range replies {
				message := tgbotapi.NewMessage(reply.ChatID, reply.Text)
				message.ReplyMarkup = reply.Markup
				if i == 0 && reply.ChatID == update.Message.Chat.ID {
					message.ReplyToMessageID = update.Message.MessageID
				}
				err = sendWithRetry(bot, message)
				if err != nil {
					log.Panicf("can't send message %+v", err)
				}
				log.Printf("sending %s %+v", message.Text, message)
			}
		case <-matchTimerChan:
			err = coffeeBot.MakeMatches(ctx, time.Now().Add(9*time.Hour))
			if err != nil {
				log.Panic("can't make matches")
			}
			time.AfterFunc(7*24*time.Hour, func() { matchTimerChan <- struct{}{} })
		case reminder := <-remindersChan:
			message := tgbotapi.NewMessage(reminder.ChatID, reminder.Text)
			err = sendWithRetry(bot, message)
			if err != nil {
				log.Panicf("can't send message %+v", err)
			}
			log.Printf("sending reminder %+v", message)
		}
	}
}

func sendWithRetry(bot *tgbotapi.BotAPI, message tgbotapi.MessageConfig) error {
	var err error
	for i := 0; i < config.SendMessageRetries; i++ {
		_, err = bot.Send(message)
		if err == nil {
			return nil
		}
		log.Printf("error sending message: %+v, sleeping and retrying", err)
		time.Sleep(config.SendMessageRetryTimeoutMs * time.Millisecond)
	}

	return err
}

func ChooseNextMatchTimerDate(clock clock.Clock) time.Time {
	tomorrow := clock.Now().AddDate(0, 0, 1)
	date := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, tomorrow.Location())
	for date.Weekday() != config.SchedulingDay {
		date = date.AddDate(0, 0, 1)
	}
	return date
}

func InitMatchTimerChan(clock clock.Clock) chan struct{} {
	date := ChooseNextMatchTimerDate(clock)
	channel := make(chan struct{})
	time.AfterFunc(date.Sub(clock.Now()), func() { channel <- struct{}{} })
	return channel
}
