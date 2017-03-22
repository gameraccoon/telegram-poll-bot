package main

import (
	"strings"
	"github.com/gameraccoon/telegram-poll-bot/database"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/nicksnyder/go-i18n/i18n"
)

func processAddQuestion(bot *tgbotapi.BotAPI, db *database.Database) {
	//if !db.IsUserEditingQuestion(userId) {
	//	db.StartCreatingQuestion(userId)
	//}
}

func processUpdate(update *tgbotapi.Update, bot *tgbotapi.BotAPI, db *database.Database, userStates map[int64]userState) {
	message := update.Message.Text
	chatId := update.Message.Chat.ID
	userId := db.GetUserId(chatId)

	T, _ := i18n.Tfunc("en-US")

	if strings.HasPrefix(message, "/") {
		switch message {
		case "/add_question":
			if !db.IsUserEditingQuestion(userId) {
				db.StartCreatingQuestion(userId)
			}
			userStates[chatId] = WaitingText
			sendMessage(bot, chatId, T("ask_question_text"))
		case "/set_text":
			if db.IsUserEditingQuestion(userId) {
				userStates[chatId] = WaitingText
				sendMessage(bot, chatId, T("ask_question_text"))
			} else {
				sendMessage(bot, chatId, T("warn_not_editing_question"))
			}
		case "/set_variants":
			if db.IsUserEditingQuestion(userId) {
				userStates[chatId] = WaitingVariants
				sendMessage(bot, chatId, T("ask_variants"))
			} else {
				sendMessage(bot, chatId, T("warn_not_editing_question"))
			}
		case "/set_rules":
			if db.IsUserEditingQuestion(userId) {
				userStates[chatId] = WaitingRules
				sendMessage(bot, chatId, T("ask_rules"))
			} else {
				sendMessage(bot, chatId, T("warn_not_editing_question"))
			}
		case "/commit_question":
			if db.IsUserEditingQuestion(userId) {
				questionId := db.GetUserEditingQuestion(userId)
				db.CommitQuestion(questionId)
				sendMessage(bot, chatId, T("say_question_commited"))
			} else {
				sendMessage(bot, chatId, T("warn_not_editing_question"))
			}
		default:
			sendMessage(bot, chatId, T("warn_unknown_command"))
		}
	} else {
		if userState, ok := userStates[chatId]; ok {
			questionId := db.GetUserEditingQuestion(userId)
			switch userState {
			case WaitingText:
				db.SetQuestionText(questionId, message)
				sendMessage(bot, chatId, T("say_text_is_set"))
			case WaitingRules:
				db.SetQuestionRules(questionId, 0, 5, 0)
				sendMessage(bot, chatId, T("say_rules_is_set"))
			case WaitingVariants:
				db.SetQuestionVariants(questionId, []string{message})
				sendMessage(bot, chatId, T("say_variants_is_set"))
			default:
				sendMessage(bot, chatId, T("warn_unknown_command"))
			}
			delete(userStates, chatId)
		} else {
			sendMessage(bot, chatId, T("warn_unknown_command"))
		}
	}
}

