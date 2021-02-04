package config

import (
	"log"
	"time"

	"yandexschooldating/messagestrings"
)

const (
	Database     = "yandexdating"
	MongoTimeout = 30 * time.Second

	SchedulingDay = time.Monday
	NotifyBefore  = time.Hour

	SendMessageRetries = 5
	// SendMessageRetryTimeoutMs
	// Yes, this is a timeout in a single-threaded code so
	// failure to send a message will block the whole bot for SendMessageRetryTimeoutMs milliseconds
	SendMessageRetryTimeoutMs = 200

	AdminUser = "riazanovskiy"
)

var MongoUri string = "mongodb://mongo:27017"

func loadLocationOrPanic(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		log.Panicf("can't load location %s", name)
	}
	return location
}

var CitiesLocation = map[string]*time.Location{
	messagestrings.Moscow:         loadLocationOrPanic("Europe/Moscow"),
	messagestrings.StPetersburg:   loadLocationOrPanic("Europe/Moscow"),
	messagestrings.Minsk:          loadLocationOrPanic("Europe/Minsk"),
	messagestrings.Novosibirsk:    loadLocationOrPanic("Asia/Novosibirsk"),
	messagestrings.Yekaterinburg:  loadLocationOrPanic("Asia/Yekaterinburg"),
	messagestrings.NizhnyNovgorod: loadLocationOrPanic("Europe/Moscow"),
	messagestrings.London:         loadLocationOrPanic("Europe/London"),
}