package messagestrings

const (
	RemindMe     = "Напомнить о встрече"
	StopMeetings = "Отказаться"
	ChangeTime   = "Изменить время"
	Activate     = "Снова участвовать"

	DefaultReply            = "у меня лапки"
	GreetingAskCity         = "Привет! В каком городе ты живёшь?"
	Welcome                 = "Теперь ты — участник встреч Random Coffee️\n\nСвою пару для встречи ты будешь узнавать каждый понедельник — сообщение придёт от имени бота. Вы пишете друг другу в Telegram, чтобы договориться, когда и как вы созвонитесь или встретитесь. Чтобы узнать партнёра на эту неделю или получить напоминание о встрече за час до неё, нажми \"" + RemindMe + "\""
	SorryNoUsername         = "Робот не работает без юзернейма в Телеграме. Установив юзернейм, нажми на /start ещё раз"
	NoMeetingsThisWeek      = "У тебя нет встречи на эту неделю"
	CouldNotFindMatch       = "К сожалению, на эту неделю встречи не нашлось"
	CouldNotParseTime       = "Не получилось распарсить время"
	ThisWeekMeetingTemplate = "На этой неделе у тебя встреча с @%s"
	TimeInThePast           = "Это время уже прошло!"
	PartnerRefused          = "К сожалению, твой партнёр отказался от встречи"
	InactiveUser            = "Ты не участвуешь в Random Coffee. Чтобы вернуться, напиши \"" + Activate + "\""
	AlreadyActive           = "Ты уже участвуешь в Random Coffee"
	NowActive               = "Теперь ты участвуешь в Random Coffee️"

	// do not modify city names. they are stored in the db

	Moscow         = "Москва"
	StPetersburg   = "Санкт-Петербург"
	Minsk          = "Минск"
	Novosibirsk    = "Новосибирск"
	Yekaterinburg  = "Екатеринбург"
	NizhnyNovgorod = "Нижний Новгород"
	London         = "Лондон"
	TelAviv        = "Тель-Авив"
	Yerevan        = "Ереван"
	Tbilisi        = "Тбилиси"
	NewYork        = "Нью-Йорк"
	Berlin         = "Берлин"
	Zurich         = "Цюрих"
	Istanbul       = "Стамбул"
)
