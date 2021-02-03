package messagestrings

const (
	RemindMe     string = "Напомнить о встрече"
	StopMeetings string = "Отказаться"
	ChangeTime   string = "Изменить время"
	Activate     string = "Снова участвовать"

	DefaultReply            string = "у меня лапки"
	GreetingAskCity         string = "Привет! В каком городе ты живёшь?"
	Welcome                 string = "Теперь ты — участник встреч Random Coffee️"
	SorryNoUsername         string = "Робот не работает без юзернейма в Телеграме. Установив юзернейм, нажми на /start ещё раз"
	NoMeetingsThisWeek      string = "У тебя нет встречи на эту неделю"
	CouldNotFindMatch       string = "К сожалению, на эту неделю встречи не нашлось"
	CouldNotParseTime       string = "Не получилось распарсить время"
	ThisWeekMeetingTemplate string = "На этой неделе у тебя встреча с @%s"
	TimeInThePast           string = "Это время уже прошло!"

	// do not modify city names. they are stored in the db

	Moscow         string = "Москва"
	StPetersburg   string = "Санкт-Петербург"
	Minsk          string = "Минск"
	Novosibirsk    string = "Новосибирск"
	Yekaterinburg  string = "Екатеринбург"
	NizhnyNovgorod string = "Нижний Новгород"
	London         string = "Лондон"
)
