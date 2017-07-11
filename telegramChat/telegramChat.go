package telegramChat

import (
	"bytes"
	"fmt"
	"github.com/gameraccoon/telegram-poll-bot/database"
	"github.com/gameraccoon/telegram-poll-bot/dialog"
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

type TelegramChat struct {
	bot *tgbotapi.BotAPI
}

func MakeTelegramChat(apiToken string) (bot TelegramChat, outErr error) {
	newBot, err := tgbotapi.NewBotAPI(apiToken)
	if err != nil {
		outErr = err
		return
	}
	
	bot = TelegramChat {
		bot: newBot,
	}
	
	return
}

func (telegramChat *TelegramChat) GetBot() *tgbotapi.BotAPI {
	return telegramChat.bot
}

func (telegramChat *TelegramChat) SetDebugModeEnabled(isEnabled bool) {
	telegramChat.bot.Debug = isEnabled
}

func (telegramChat *TelegramChat) GetBotUsername() string {
	return telegramChat.bot.Self.UserName
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
		telegramChat.SendMessage(chatId, message)
	}

	db.UnmarkUsersReady(usersChatIds)
}

func (telegramChat *TelegramChat) SendDialog(dialog *dialog.Dialog, chatId int64) {
	
}
