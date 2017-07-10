package chat

import (
	"github.com/gameraccoon/telegram-poll-bot/database"
	"github.com/gameraccoon/telegram-poll-bot/processing"
)

type Сhat interface {
	SendMessage(chatId int64, message string)
	SendQuestion(db *database.Database, questionId int64, usersChatIds []int64)
	SendDialog(data *processing.ProcessData, dialog *Dialog, chatId int64)
}
