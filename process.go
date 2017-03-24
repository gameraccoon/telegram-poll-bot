package main

import (
	"fmt"
	"strings"
	"strconv"
	"github.com/gameraccoon/telegram-poll-bot/database"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/nicksnyder/go-i18n/i18n"
)

func setVariants(db *database.Database, questionId int64, message string) (ok bool) {
	variants := strings.Split(message, "\n")
	db.SetQuestionVariants(questionId, variants)
	return true
}

func setRules(db *database.Database, questionId int64, message string) (ok bool) {
	rules := strings.Split(message, " ")
	if len(rules) != 3 {
		return false
	}

	min_answers, err1 := strconv.ParseInt(rules[0], 10, 64)
	if err1 != nil {
		return false
	}

	max_answers, err2 := strconv.ParseInt(rules[1], 10, 64)
	if err2 != nil {
		return false
	}

	time, err3 := strconv.ParseInt(rules[2], 10, 64)
	if err3 != nil {
		return false
	}

	db.SetQuestionRules(questionId, min_answers, max_answers, time)
	return true
}

func sendQuestion(bot *tgbotapi.BotAPI, db *database.Database, questionId int64, usersChatIds []int64) {
	questionMessage := db.GetQuestionText(questionId) + "\n"

	variants := db.GetQuestionVariants(questionId)
	for i, variant := range(variants) {
		questionMessage = questionMessage + fmt.Sprintf("/ans%d - %s\n", i, variant)
	}

	questionMessage = questionMessage + "/skip"

	for _, chatId := range(usersChatIds) {
		sendMessage(bot, chatId, questionMessage)
	}

	db.UnmarkUsersReady(usersChatIds)
}

func processNextQuestion(bot *tgbotapi.BotAPI, db *database.Database, userId int64, chatId int64) {
	if db.IsUserHasPendingQuestions(userId) {
		nextQuestion := db.GetUserNextQuestion(userId)
		sendQuestion(bot, db, nextQuestion, []int64{chatId})
	} else {
		db.MarkUserReady(userId)
	}
}

func commitQuestion(bot *tgbotapi.BotAPI, db *database.Database, userId int64, chatId int64, questionId int64) {
	T, _ := i18n.Tfunc("en-US")
	db.CommitQuestion(questionId)
	sendMessage(bot, chatId, T("say_question_commited"))

	processNextQuestion(bot, db, userId, chatId)

	users := db.GetReadyUsersChatIds()

	sendQuestion(bot, db, questionId, users)
}

func sendResults(bot *tgbotapi.BotAPI, db *database.Database, questionId int64, respondents []int64) {
	resultText := db.GetQuestionText(questionId)

	variants := db.GetQuestionVariants(questionId)
	answers := db.GetQuestionAnswers(questionId)
	answersCount := db.GetQuestionAnswersCount(questionId)

	for i, variant := range(variants) {
		resultText = resultText + fmt.Sprintf("\n%s - %d(%d%%)", variant, answers[i], int64(100.0*float32(answers[i])/float32(answersCount)))
	}

	for _, respondent := range(respondents) {
		sendMessage(bot, respondent, resultText)
	}
}

func completeQuestion(bot *tgbotapi.BotAPI, db *database.Database, questionId int64) {
	T, _ := i18n.Tfunc("en-US")
	db.EndQuestion(questionId)

	users := db.GetUsersAnsweringQuestionNow(questionId)
	for _, user := range(users) {
		db.RemoveUserPendingQuestion(user, questionId)
		chatId := db.GetUserChatId(user)
		sendMessage(bot, db.GetUserChatId(user), T("say_question_outdated"))

		if db.IsUserHasPendingQuestions(user) {
			sendQuestion(bot, db, db.GetUserNextQuestion(user), []int64{chatId})
		} else {
			db.MarkUserReady(user)
		}
	}

	db.RemoveQuestionFromAllUsers(questionId)

	respondents := db.GetQuestionRespondents(questionId)
	sendResults(bot, db, questionId, respondents)
}

func processCompleteness(bot *tgbotapi.BotAPI, db *database.Database, questionId int64) {
	_, max_answers, _ := db.GetQuestionRules(questionId)

	answersCount := db.GetQuestionAnswersCount(questionId)

	if answersCount >= max_answers {
		completeQuestion(bot, db, questionId)
		return
	}

	if db.GetQuestionPendingCount(questionId) == 0 {
		completeQuestion(bot, db, questionId)
		return
	}
}

func parseAnswer(bot *tgbotapi.BotAPI, db *database.Database, chatId int64, userId int64, message string) {
	T, _ := i18n.Tfunc("en-US")
	questionId := db.GetUserNextQuestion(userId)
	variantsCount := db.GetQuestionVariantsCount(questionId)

	if message == "/skip" {
		db.RemoveUserPendingQuestion(userId, questionId)
		sendMessage(bot, chatId, T("say_question_skipped"))

		processCompleteness(bot, db, questionId)

		processNextQuestion(bot, db, userId, chatId)
		return
	}

	if !strings.HasPrefix(message, "/ans") {
		sendMessage(bot, chatId, T("warn_wrong_answer"))
		return
	}

	answer, err := strconv.ParseInt(message[4:len(message)], 10, 64)
	if err != nil {
		sendMessage(bot, chatId, T("warn_wrong_answe"))
		return
	}

	if answer >= 0 && answer < variantsCount {
		db.AddQuestionAnswer(questionId, userId, answer)
		db.RemoveUserPendingQuestion(userId, questionId)
		sendMessage(bot, chatId, T("say_answer_added"))

		processCompleteness(bot, db, questionId)

		processNextQuestion(bot, db, userId, chatId)
	} else {
		sendMessage(bot, chatId, T("warn_wrong_answer"))
	}
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
				db.UnmarkUserReady(userId)
				userStates[chatId] = WaitingText
				sendMessage(bot, chatId, T("ask_question_text"))
			} else {
				sendMessage(bot, chatId, T("editing_commands"))
			}
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
				if db.IsQuestionReady(questionId) && db.GetQuestionVariantsCount(questionId) > 0 {
					commitQuestion(bot, db, userId, chatId, questionId)
				} else {
					sendMessage(bot, chatId, T("warn_question_not_ready"))
				}
			} else {
				sendMessage(bot, chatId, T("warn_not_editing_question"))
			}
		case "/discard_question":
			if db.IsUserEditingQuestion(userId) {
				questionId := db.GetUserEditingQuestion(userId)
				db.DiscardQuestion(questionId)
				sendMessage(bot, chatId, T("say_question_discarded"))
			} else {
				sendMessage(bot, chatId, T("warn_not_editing_question"))
			}
		default:
			if !db.IsUserEditingQuestion(userId) && db.IsUserHasPendingQuestions(userId) {
				parseAnswer(bot, db, chatId, userId, message)
			} else {
				sendMessage(bot, chatId, T("warn_unknown_command"))
			}
		}
	} else {
		if userState, ok := userStates[chatId]; ok {
			questionId := db.GetUserEditingQuestion(userId)
			switch userState {
			case WaitingText:
				db.SetQuestionText(questionId, message)
				sendMessage(bot, chatId, T("say_text_is_set"))
				sendMessage(bot, chatId, T("editing_commands"))
				delete(userStates, chatId)
			case WaitingVariants:
				ok := setVariants(db, questionId, message)
				if ok {
					sendMessage(bot, chatId, T("say_variants_is_set"))
					sendMessage(bot, chatId, T("editing_commands"))
					delete(userStates, chatId)
				} else {
					sendMessage(bot, chatId, T("warn_bad_variants"))
				}
			case WaitingRules:
				ok := setRules(db, questionId, message)
				if ok {
					sendMessage(bot, chatId, T("say_rules_is_set"))
					sendMessage(bot, chatId, T("editing_commands"))
					delete(userStates, chatId)
				} else {
					sendMessage(bot, chatId, T("warn_bad_rules"))
				}

			default:
				sendMessage(bot, chatId, T("warn_unknown_command"))
				delete(userStates, chatId)
			}
		} else {
			sendMessage(bot, chatId, T("warn_unknown_command"))
		}
	}
}

