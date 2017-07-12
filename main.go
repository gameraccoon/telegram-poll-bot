package main

import (
	"encoding/json"
	"github.com/gameraccoon/telegram-poll-bot/database"
	"github.com/gameraccoon/telegram-poll-bot/dialogFactories"
	"github.com/gameraccoon/telegram-poll-bot/processing"
	"github.com/gameraccoon/telegram-poll-bot/telegramChat"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/nicksnyder/go-i18n/i18n"
	"io/ioutil"
	"log"
	"strings"
	"sync"
	"time"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	i18n.MustLoadTranslationFile("./data/strings/en-us.all.json")
	i18n.MustLoadTranslationFile("./data/strings/ru-ru.all.json")
}

func getFileStringContent(filePath string) (content string, err error) {
	fileContent, err := ioutil.ReadFile(filePath)
	if err == nil {
		content = strings.TrimSpace(string(fileContent))
	}
	return
}

func getApiToken() (token string, err error) {
	return getFileStringContent("./telegramApiToken.txt")
}

func loadConfig(path string) (config processing.StaticConfiguration, err error) {
	jsonString, err := getFileStringContent(path)
	if err == nil {
		dec := json.NewDecoder(strings.NewReader(jsonString))
		err = dec.Decode(&config)
	}
	return
}

func updateTimers(staticData *processing.StaticProccessStructs, mutex *sync.Mutex) {
	questions := staticData.Db.GetActiveQuestions()

	mutex.Lock()
	for _, questionId := range questions {
		_, _, endTime := staticData.Db.GetQuestionRules(questionId)
		if endTime > 0 {
			staticData.Timers[questionId] = time.Unix(endTime, 0)
		}
	}
	mutex.Unlock()

	for {
		currentTime := time.Now()
		mutex.Lock()
		for questionId, endTime := range staticData.Timers {
			if endTime.Sub(currentTime).Seconds() < 0.0 {
				delete(staticData.Timers, questionId)
				processTimer(staticData, questionId)
			}
		}
		mutex.Unlock()
		time.Sleep(30 * time.Second)
	}
}

func updateBot(bot *tgbotapi.BotAPI, staticData *processing.StaticProccessStructs, dialogManager *dialogFactories.DialogManager, mutex *sync.Mutex) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	if err != nil {
		log.Fatal(err.Error())
	}

	staticData.Processors = makeUserCommandProcessors()
	staticData.ModeratorProcessors = makeModeratorCommandProcessors()

	for update := range updates {
		if update.Message == nil {
			continue
		}
		mutex.Lock()
		processUpdate(&update, staticData, dialogManager)
		mutex.Unlock()
	}
}

func main() {
	apiToken, err := getApiToken()
	if err != nil {
		log.Fatal(err.Error())
	}

	config, err := loadConfig("./config.json")
	if err != nil {
		log.Fatal(err.Error())
	}

	t, err := i18n.Tfunc(config.Language)
	if err != nil {
		log.Fatal(err.Error())
	}

	db := &database.Database{}
	err = db.Connect("./polls-data.db")
	defer db.Disconnect()

	if err != nil {
		log.Fatal("Can't connect database")
	}

	database.UpdateVersion(db)

	userStates := make(map[int64]processing.UserState)

	timers := make(map[int64]time.Time)

	mutex := &sync.Mutex{}

	chat, err := telegramChat.MakeTelegramChat(apiToken)
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Printf("Authorized on account %s", chat.GetBotUsername())

	chat.SetDebugModeEnabled(config.ExtendedLog)

	dialogManager := &(dialogFactories.DialogManager{})
	dialogManager.RegisterDialogFactory("ed", dialogFactories.MakeQuestionEditDialogFactory())

	staticData := &processing.StaticProccessStructs{
		Chat:       chat,
		Db:         db,
		Config:     &config,
		Timers:     timers,
		Trans:      t,
		UserStates: userStates,
	}

	go updateTimers(staticData, mutex)
	updateBot(chat.GetBot(), staticData, dialogManager, mutex)
}
