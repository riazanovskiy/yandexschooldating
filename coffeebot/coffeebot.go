package coffeebot

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"yandexschooldating/clock"
	"yandexschooldating/config"
	"yandexschooldating/match"
	"yandexschooldating/messagestrings"
	"yandexschooldating/reminder"
	"yandexschooldating/user"
	"yandexschooldating/util"

	"github.com/joomcode/errorx"
)

type userState struct {
	waitingForCity bool
	waitingForDate bool
	lastMarkup     interface{}
}

type CoffeeBot struct {
	userDAO     user.DAO
	matchDAO    match.DAO
	reminderDAO reminder.DAO

	clock clock.Clock

	removeMarkup                         interface{}
	citiesKeyboard                       interface{}
	remindStopMeetingsKeyboard           interface{}
	remindChangeTimeStopMeetingsKeyboard interface{}
	activateKeyboard                     interface{}

	state map[int]*userState
}

type BotReply struct {
	ChatID int64
	Text   string
	Markup interface{}
}

func NewCoffeeBot(
	userDAO user.DAO,
	matchDAO match.DAO,
	reminderDAO reminder.DAO,
	clock clock.Clock,
	removeMarkup interface{},
	citiesKeyboard interface{},
	remindStopMeetingsKeyboard interface{},
	remindChangeTimeStopMeetingsKeyboard interface{},
	activateKeyboard interface{},
) *CoffeeBot {
	return &CoffeeBot{
		userDAO:                              userDAO,
		matchDAO:                             matchDAO,
		reminderDAO:                          reminderDAO,
		clock:                                clock,
		removeMarkup:                         removeMarkup,
		citiesKeyboard:                       citiesKeyboard,
		remindStopMeetingsKeyboard:           remindStopMeetingsKeyboard,
		remindChangeTimeStopMeetingsKeyboard: remindChangeTimeStopMeetingsKeyboard,
		activateKeyboard:                     activateKeyboard,
		state:                                make(map[int]*userState),
	}
}

func (b *CoffeeBot) findUserByID(ctx context.Context, ID int) (*user.User, error) {
	user, err := b.userDAO.FindUserByID(ctx, ID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errorx.IllegalState.New("can't find user %d", ID)
	}
	return user, nil
}

func formatMatchMessage(thisUser *user.User, otherUser *user.User, meetingTime time.Time) string {
	message := "Встреча с @" + otherUser.Username + " будет " + meetingTime.In(util.GetLocationForCityOrUTC(thisUser.City)).Format("02 January в 15:04 MST")
	if thisUser.City != otherUser.City {
		message = "Встречи в твоём городе не нашлось. " + message
	}
	return message
}

func (b *CoffeeBot) getMatchOrNoMeetingsReply(ctx context.Context, userID int, chatID int64) (*match.Match, []BotReply, error) {
	match, err := b.matchDAO.FindCurrentMatchForUserID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if match == nil {
		b.setLastMarkup(userID, b.remindStopMeetingsKeyboard)
		return nil, []BotReply{{chatID, messagestrings.NoMeetingsThisWeek, b.getLastMarkup(userID)}}, nil
	}
	return match, nil, nil
}

func (b *CoffeeBot) ProcessMessage(ctx context.Context, userID int, username string, chatID int64, text string) ([]BotReply, error) {
	if b.state[userID] == nil {
		b.state[userID] = &userState{lastMarkup: b.remindStopMeetingsKeyboard}
	}

	if len(username) == 0 {
		return []BotReply{{chatID, messagestrings.SorryNoUsername, b.removeMarkup}}, nil
	}

	switch text {
	case "/start":
		b.state[userID].waitingForCity = true
		return []BotReply{{chatID, messagestrings.GreetingAskCity, b.citiesKeyboard}}, nil
	case messagestrings.RemindMe:
		match, replies, err := b.getMatchOrNoMeetingsReply(ctx, userID, chatID)
		if err != nil || replies != nil {
			return replies, err
		}
		otherUser, err := b.findUserByID(ctx, match.SecondID)
		if err != nil {
			return nil, err
		}

		thisUser, err := b.findUserByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		var reply string
		if match.MeetingTime == nil {
			reply = fmt.Sprintf("У тебя встреча с @%s. Чтобы получить сообщение перед встречей, напиши время встречи в формате число.месяц часы:минуты, например 02.01 15:04", otherUser.Username)
			_, ok := config.CitiesLocation[thisUser.City]
			if !ok {
				reply += ". Поскольку мы не знаем часового пояса для твоего города, время должно быть в формате UTC"
			}
			b.state[userID].waitingForDate = true
			b.setLastMarkup(userID, b.removeMarkup)
		} else {
			reply = formatMatchMessage(thisUser, otherUser, *match.MeetingTime)
			b.setLastMarkup(userID, b.remindChangeTimeStopMeetingsKeyboard)
		}
		return []BotReply{{chatID, reply, b.getLastMarkup(userID)}}, nil
	case "MakeMatches":
		if username == config.AdminUser {
			err := b.MakeMatches(ctx, b.clock.Now().Add(30*time.Second))
			var reply string
			if err == nil {
				reply = "MakeMatches succeeded"
			} else {
				reply = "MakeMatches error: " + err.Error()
			}
			return []BotReply{{chatID, reply, b.getLastMarkup(userID)}}, nil
		}
	default:
		switch {
		case b.state[userID].waitingForCity:
			b.state[userID].waitingForCity = false
			err := b.userDAO.UpsertUser(ctx, userID, username, text, chatID, true)
			if err != nil {
				return nil, err
			}
			return []BotReply{{chatID, messagestrings.Welcome, b.getLastMarkup(userID)}}, nil
		case b.state[userID].waitingForDate:
			b.state[userID].waitingForDate = false
			match, reply, err := b.getMatchOrNoMeetingsReply(ctx, userID, chatID)
			if err != nil || reply != nil {
				return reply, err
			}
			thisUser, err := b.findUserByID(ctx, userID)
			if err != nil {
				return nil, err
			}
			parsedTime, err := time.ParseInLocation("02.01 15:04", text, util.GetLocationForCityOrUTC(thisUser.City))
			if err == nil {
				meetingTime := time.Date(
					b.clock.Now().Year(),
					parsedTime.Month(),
					parsedTime.Day(),
					parsedTime.Hour(),
					parsedTime.Minute(),
					parsedTime.Second(),
					0,
					parsedTime.Location(),
				)
				err = b.matchDAO.UpdateMatchTime(ctx, userID, meetingTime)
				if err != nil {
					return nil, err
				}

				b.setLastMarkup(userID, b.remindChangeTimeStopMeetingsKeyboard)
				b.setLastMarkup(match.SecondID, b.remindChangeTimeStopMeetingsKeyboard)

				if int(meetingTime.Sub(b.clock.Now()).Seconds()) <= 1 {
					return []BotReply{{thisUser.ChatID, messagestrings.TimeInThePast, b.getLastMarkup(userID)}}, nil
				}

				otherUser, err := b.findUserByID(ctx, match.SecondID)
				if err != nil {
					return nil, err
				}

				thisMessage := formatMatchMessage(thisUser, otherUser, meetingTime)
				otherMessage := formatMatchMessage(otherUser, thisUser, meetingTime)

				err = b.reminderDAO.AddReminder(ctx, meetingTime, thisUser.ChatID, thisMessage)
				if err != nil {
					return nil, err
				}
				err = b.reminderDAO.AddReminder(ctx, meetingTime, otherUser.ChatID, otherMessage)
				if err != nil {
					return nil, err
				}

				reminderTime := meetingTime.Add(-1 * config.NotifyBefore)
				if reminderTime.Sub(b.clock.Now()).Minutes() >= 1 {
					err = b.reminderDAO.AddReminder(ctx, reminderTime, thisUser.ChatID, thisMessage)
					if err != nil {
						return nil, err
					}
					err = b.reminderDAO.AddReminder(ctx, reminderTime, otherUser.ChatID, otherMessage)
					if err != nil {
						return nil, err
					}
				}
				return []BotReply{
					{thisUser.ChatID, thisMessage, b.getLastMarkup(thisUser.ID)},
					{otherUser.ChatID, otherMessage, b.getLastMarkup(otherUser.ID)},
				}, nil
			} else {
				log.Printf("error parsing date %s", text)
				b.setLastMarkup(userID, b.remindStopMeetingsKeyboard)
				return []BotReply{{chatID, messagestrings.CouldNotParseTime, b.getLastMarkup(userID)}}, nil
			}
		}
	}
	return []BotReply{{chatID, messagestrings.DefaultReply, b.getLastMarkup(userID)}}, nil
}

func (b *CoffeeBot) makeMatchesForList(ctx context.Context, reminderTime time.Time, users []user.User) error {
	for i := 0; i+1 < len(users); i += 2 {
		err := b.matchDAO.AddMatch(ctx, users[i].ID, users[i+1].ID)
		if err != nil {
			return err
		}
		b.setLastMarkup(users[i].ID, b.remindStopMeetingsKeyboard)
		b.setLastMarkup(users[i+1].ID, b.remindStopMeetingsKeyboard)
		err = b.reminderDAO.AddReminder(ctx, reminderTime, users[i].ChatID, fmt.Sprintf(messagestrings.ThisWeekMeetingTemplate, users[i+1].Username))
		if err != nil {
			return err
		}
		err = b.reminderDAO.AddReminder(ctx, reminderTime, users[i+1].ChatID, fmt.Sprintf(messagestrings.ThisWeekMeetingTemplate, users[i].Username))
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *CoffeeBot) MakeMatches(ctx context.Context, reminderTime time.Time) error {
	log.Printf("starting MakeMatches with reminderTime %s", reminderTime.String())
	activeUsers, err := b.userDAO.FindActiveUsers(ctx)
	if err != nil {
		return err
	}

	rand.Shuffle(len(activeUsers), func(i, j int) { activeUsers[i], activeUsers[j] = activeUsers[j], activeUsers[i] })

	cities := make(map[string][]user.User)
	var leftovers []user.User
	for _, user := range activeUsers {
		if user.RemoteFirst {
			leftovers = append(leftovers, user)
		} else {
			cities[user.City] = append(cities[user.City], user)
		}
	}

	b.matchDAO.IncrementMatchingCycle()

	for _, users := range cities {
		err = b.makeMatchesForList(ctx, reminderTime, users)
		if err != nil {
			return err
		}
		if len(users)%2 == 1 {
			leftovers = append(leftovers, users[len(users)-1])
		}
	}

	rand.Shuffle(len(leftovers), func(i, j int) { leftovers[i], leftovers[j] = leftovers[j], leftovers[i] })

	err = b.makeMatchesForList(ctx, reminderTime, leftovers)
	if err != nil {
		return err
	}
	if len(leftovers)%2 == 1 {
		lastUser := leftovers[len(leftovers)-1]
		b.setLastMarkup(lastUser.ID, b.remindStopMeetingsKeyboard)
		return b.reminderDAO.AddReminder(ctx, reminderTime, lastUser.ChatID, messagestrings.CouldNotFindMatch)
	}
	return nil
}

func (b *CoffeeBot) setLastMarkup(userID int, markup interface{}) {
	if b.state[userID] == nil {
		b.state[userID] = &userState{lastMarkup: markup}
	} else {
		b.state[userID].lastMarkup = markup
	}
}

func (b *CoffeeBot) getLastMarkup(userID int) interface{} {
	if b.state[userID] == nil {
		b.state[userID] = &userState{lastMarkup: b.remindStopMeetingsKeyboard}
	}
	return b.state[userID].lastMarkup
}
