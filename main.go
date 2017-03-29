package main

import (
	"fmt"
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

func getFileStringContent(filePath string) string {
	fileContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Print(err)
	}

	return strings.TrimSpace(string(fileContent))
}

func getApiToken() string {
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

func updateTimers(bot *tgbotapi.BotAPI, db *database.Database, t i18n.TranslateFunc, timers map[int64]time.Time, mutex *sync.Mutex) {
	questions := db.GetActiveQuestions()

	mutex.Lock()
	for _, questionId := range questions {
		_, _, endTime := db.GetQuestionRules(questionId)
		if endTime > 0 {
			timers[questionId] = time.Unix(endTime, 0)
		}
	}
	mutex.Unlock()

	for {
		currentTime := time.Now()
		mutex.Lock()
		for questionId, endTime := range timers {
			if endTime.Sub(currentTime).Seconds() < 0.0 {
				delete(timers, questionId)
				processTimer(bot, db, questionId, timers, t)
			}
		}
		mutex.Unlock()
		time.Sleep(30 * time.Second)
	}
}

func updateBot(bot *tgbotapi.BotAPI, db *database.Database, userStates map[int64]userState, t i18n.TranslateFunc, timers map[int64]time.Time, mutex *sync.Mutex) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	if err != nil {
		log.Fatal(err.Error())
	}

	for update := range updates {
		if update.Message == nil {
			continue
		}
		mutex.Lock()
		processUpdate(&update, bot, db, userStates, timers, t)
		mutex.Unlock()
	}
}

func main() {
	var apiToken string = getApiToken()

	bot, err := tgbotapi.NewBotAPI(apiToken)
	if err != nil {
		log.Panic(err)
	}

	//bot.Debug = true

	t, err := i18n.Tfunc("ru-RU")
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

	userStates := make(map[int64]userState)

	timers := make(map[int64]time.Time)

	mutex := &sync.Mutex{}

	go updateTimers(bot, db, t, timers, mutex)
	updateBot(bot, db, userStates, t, timers, mutex)
}
