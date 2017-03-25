package main

import (
	"fmt"
	"log"
	"strings"
	"io/ioutil"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/gameraccoon/telegram-poll-bot/database"
	"github.com/nicksnyder/go-i18n/i18n"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	i18n.MustLoadTranslationFile("./data/strings/en-us.all.json")
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

func main() {
	var apiToken string = getApiToken()

	bot, err := tgbotapi.NewBotAPI(apiToken)
	if err != nil {
		log.Panic(err)
	}

	//bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	db := &database.Database{}
	err = db.Connect("./polls-data.db")
	defer db.Disconnect()

	if err != nil {
		log.Fatal("Can't connect database")
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	userStates := make(map[int64]userState)

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		processUpdate(&update, bot, db, userStates)
	}
}
