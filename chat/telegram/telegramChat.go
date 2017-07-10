package chat.telegram

import (
	"github.com/gameraccoon/telegram-poll-bot/dialog"
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

type TelegramChat struct {
	bot *tgbotapi.BotAPI
}

func (telegramChat *TelegramChat) SendMessage(chatId int64, message string) {
	msg := tgbotapi.NewMessage(chatId, message)
	msg.ParseMode = "HTML"
	telegramChat.bot.Send(msg)
}

func (telegramChat *TelegramChat) SendQuestion(db *database.Database, questionId int64, usersChatIds []int64) {
	var buffer bytes.Buffer

	buffer.WriteString(db.GetQuestionText(questionId) + "\n")

	variants := db.GetQuestionVariants(questionId)
	for i, variant := range variants {
		buffer.WriteString(fmt.Sprintf("/ans%d - %s\n", i+1, variant))
	}

	buffer.WriteString("/skip")
	message := buffer.String()

	for _, chatId := range usersChatIds {
		SendMessage(telegramChat.bot, chatId, message)
	}

	db.UnmarkUsersReady(usersChatIds)
}

func (telegramChat *TelegramChat) SendDialog(data *processing.ProcessData, dialog *Dialog, chatId int64) {
	
}
