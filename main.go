package main

import (
	"encoding/json"
	"github.com/gameraccoon/telegram-poll-bot/database"
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

func sendMessage(bot *tgbotapi.BotAPI, chatId int64, message string) {
	msg := tgbotapi.NewMessage(chatId, message)
	msg.ParseMode = "HTML"
	bot.Send(msg)
}

type userState int

const (
	Normal userState = iota
	WaitingText
	WaitingVariants
	WaitingRules
)

type configuration struct {
	Language    string
	Moderators  []int64
	ExtendedLog bool
}

func loadConfig(path string) (config configuration, err error) {
	jsonString, err := getFileStringContent(path)
	if err == nil {
		dec := json.NewDecoder(strings.NewReader(jsonString))
		err = dec.Decode(&config)
	}
	return
}

func updateTimers(staticData *staticProccessStructs, mutex *sync.Mutex) {
	questions := staticData.db.GetActiveQuestions()

	mutex.Lock()
	for _, questionId := range questions {
		_, _, endTime := staticData.db.GetQuestionRules(questionId)
		if endTime > 0 {
			staticData.timers[questionId] = time.Unix(endTime, 0)
		}
	}
	mutex.Unlock()

	for {
		currentTime := time.Now()
		mutex.Lock()
		for questionId, endTime := range staticData.timers {
			if endTime.Sub(currentTime).Seconds() < 0.0 {
				delete(staticData.timers, questionId)
				processTimer(staticData, questionId)
			}
		}
		mutex.Unlock()
		time.Sleep(30 * time.Second)
	}
}

func updateBot(staticData *staticProccessStructs, mutex *sync.Mutex) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := staticData.bot.GetUpdatesChan(u)

	if err != nil {
		log.Fatal(err.Error())
	}

	staticData.processors = makeUserCommandProcessors()
	staticData.moderatorProcessors = makeModeratorCommandProcessors()

	for update := range updates {
		if update.Message == nil {
			continue
		}
		mutex.Lock()
		processUpdate(&update, staticData)
		mutex.Unlock()
	}
}

func main() {
	apiToken, err := getApiToken()
	if err != nil {
		log.Fatal(err.Error())
	}

	bot, err := tgbotapi.NewBotAPI(apiToken)
	if err != nil {
		log.Fatal(err.Error())
	}

	config, err := loadConfig("./config.json")
	if err != nil {
		log.Fatal(err.Error())
	}

	bot.Debug = config.ExtendedLog

	t, err := i18n.Tfunc(config.Language)
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	db := &database.Database{}
	err = db.Connect("./polls-data.db")
	defer db.Disconnect()

	if err != nil {
		log.Fatal("Can't connect database")
	}

	database.UpdateVersion(db)

	userStates := make(map[int64]userState)

	timers := make(map[int64]time.Time)

	mutex := &sync.Mutex{}

	staticData := &staticProccessStructs{
		bot:        bot,
		db:         db,
		config:     &config,
		timers:     timers,
		trans:      t,
		userStates: userStates,
	}

	go updateTimers(staticData, mutex)
	updateBot(staticData, mutex)
}
