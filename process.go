package main

import (
	"strings"
	"github.com/gameraccoon/telegram-poll-bot/database"
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

func processAddQuestion(bot *tgbotapi.BotAPI, db *database.Database) {
	//if !db.IsUserEditingQuestion(userId) {
	//	db.StartCreatingQuestion(userId)
	//}
}

func processUpdate(update *tgbotapi.Update, bot *tgbotapi.BotAPI, db *database.Database) {
	message := update.Message.Text
	//userId := db.GetUserId(update.Message.Chat.ID)

	if strings.HasPrefix(message, "/add_question ") {
		processAddQuestion(bot, db)
		sendMessage(bot, update.Message.Chat.ID, "Add Question")
	} else if strings.HasPrefix(message, "/set_answers\n") {
		sendMessage(bot, update.Message.Chat.ID, "Set Answers")

	} else if strings.HasPrefix(message, "/set_rules ") {
		sendMessage(bot, update.Message.Chat.ID, "Set Rules")

	} else if message == "/commit_question" {
		sendMessage(bot, update.Message.Chat.ID, "Commit Question")

	} else if strings.HasPrefix(message, "/") {
		sendMessage(bot, update.Message.Chat.ID, "Answer")

	} else {
		sendMessage(bot, update.Message.Chat.ID, "Uncnown command")
	}
}

