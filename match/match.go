package match

import (
	"context"
	"time"

	"yandexschooldating/clock"

	"github.com/joomcode/errorx"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Match struct {
	FirstID       int        `bson:"firstId"`
	SecondID      int        `bson:"secondId"`
	MatchUnixTime int64      `bson:"matchUnixTime"`
	MeetingTime   *time.Time `bson:"meetingTime"`
	Refused       bool       `bson:"refused"`
	MatchingCycle int        `bson:"matchingCycle"`
}

//goland:noinspection GoNameStartsWithPackageName
var MatchBSON = struct {
	FirstID       string
	SecondID      string
	MatchUnixTime string
	MeetingTime   string
	Refused       string
	MatchingCycle string
}{
	"firstId",
	"secondId",
	"matchUnixTime",
	"meetingTime",
	"refused",
	"matchingCycle",
}

type DAO interface {
	FindCurrentMatchForUserID(ctx context.Context, userID int) (*Match, error)
	AddMatch(ctx context.Context, firstID, secondID int) error
	UpdateMatchTime(ctx context.Context, ID int, time time.Time) error
	InitializeMatchingCycle(ctx context.Context) error
	IncrementMatchingCycle()
}

type dao struct {
	matches       *mongo.Collection
	clock         clock.Clock
	matchingCycle int
}

var _ DAO = (*dao)(nil)

func NewDAO(client *mongo.Client, database string, clock clock.Clock) DAO {
	return &dao{matches: client.Database(database).Collection("matches"), clock: clock, matchingCycle: 0}
}

// InitializeMatchingCycle It is most likely a mistake to use MatchDAO without a call to InitializeMatchingCycle
func (m *dao) InitializeMatchingCycle(ctx context.Context) error {
	cursor, err := m.matches.Find(ctx, bson.M{}, options.Find().SetLimit(1).SetSort(bson.M{MatchBSON.MatchingCycle: -1}))
	if err != nil {
		return errorx.Decorate(err, "error initializing matching cycle")
	}
	if cursor.Next(ctx) {
		var document Match
		err = cursor.Decode(&document)
		if err != nil {
			return err
		}
		m.matchingCycle = document.MatchingCycle
	}
	return nil
}

func (m *dao) IncrementMatchingCycle() {
	m.matchingCycle++
}

func (m *dao) filterBson(userID int) bson.M {
	return bson.M{"$or": []bson.M{
		{MatchBSON.FirstID: userID, MatchBSON.MatchingCycle: m.matchingCycle, MatchBSON.Refused: false},
		{MatchBSON.SecondID: userID, MatchBSON.MatchingCycle: m.matchingCycle, MatchBSON.Refused: false},
	}}
}

func (m *dao) UpdateMatchTime(ctx context.Context, userID int, meetingTime time.Time) error {
	result, err := m.matches.UpdateOne(ctx, m.filterBson(userID), bson.M{"$set": bson.M{MatchBSON.MeetingTime: meetingTime}})
	if err != nil {
		return errorx.Decorate(err, "error updating meeting time")
	}
	if result.MatchedCount == 0 {
		return errorx.IllegalArgument.New("no current match for user %d", userID)
	}
	return nil
}

// FindCurrentMatchForUserID always returns a match with its FirstID set to the userUD
func (m *dao) FindCurrentMatchForUserID(ctx context.Context, userID int) (*Match, error) {
	result := m.matches.FindOne(ctx, m.filterBson(userID))
	if result.Err() == mongo.ErrNoDocuments {
		return nil, nil
	}
	if result.Err() != nil {
		return nil, errorx.Decorate(result.Err(), "can't find match for user %d", userID)
	}

	var match Match
	err := result.Decode(&match)
	if err != nil {
		return nil, errorx.Decorate(err, "can't decode match")
	}

	if match.SecondID == userID {
		match.FirstID, match.SecondID = match.SecondID, match.FirstID
	}

	return &match, nil
}

func (m *dao) checkExistingMatch(ctx context.Context, userID int) error {
	oldMatch, err := m.FindCurrentMatchForUserID(ctx, userID)
	if err != nil {
		return errorx.Decorate(err, "can't check for existing matches for user %d", userID)
	}
	if oldMatch != nil {
		return errorx.IllegalArgument.New("match exists for user %d", userID)
	}
	return nil
}

func (m *dao) AddMatch(ctx context.Context, firstID, secondID int) error {
	err := m.checkExistingMatch(ctx, firstID)
	if err != nil {
		return err
	}
	err = m.checkExistingMatch(ctx, secondID)
	if err != nil {
		return err
	}
	match := Match{
		FirstID:       firstID,
		SecondID:      secondID,
		MatchUnixTime: m.clock.Now().Unix(),
		MatchingCycle: m.matchingCycle,
		Refused:       false,
	}
	_, err = m.matches.InsertOne(ctx, match)
	return err
}
