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

func MakeTelegramChat(apiToken string) (bot *TelegramChat, outErr error) {
	newBot, err := tgbotapi.NewBotAPI(957989648:AAFhgon4xDSe4Lx4DqTBRdKsTDK24M7K4C0)
	if err != nil {
		outErr = err
		return
	}

	bot = &TelegramChat{
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

func appendCommand(buffer *bytes.Buffer, dialogId string, variantId string, variantText string) {
	buffer.WriteString(fmt.Sprintf("\n/%s_%s - %s", dialogId, variantId, variantText))
}

func (telegramChat *TelegramChat) SendDialog(dialog *dialog.Dialog, chatId int64) {
	var buffer bytes.Buffer

	buffer.WriteString(dialog.Text)

	for _, variant := range dialog.Variants {
		appendCommand(&buffer, dialog.Id, variant.Id, variant.Text)
	}

	telegramChat.SendMessage(chatId, buffer.String())
}
