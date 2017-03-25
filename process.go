package main

import (
	"fmt"
	"strings"
	"strconv"
	"bytes"
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
	if len(rules) == 0 {
		return false
	}

	var min_answers int64
	var max_answers int64
	var time int64
	var err error

	if len(rules) >= 1 {
		min_answers, err = strconv.ParseInt(rules[0], 10, 64)
		if err != nil || min_answers < 0 {
			return false
		}
	}

	if len(rules) >= 2 {
		max_answers, err = strconv.ParseInt(rules[1], 10, 64)
		if err != nil || max_answers < 0 {
			return false
		}
	}

	if len(rules) >= 3 {
		time, err = strconv.ParseInt(rules[2], 10, 64)
		if err != nil || time < 0 {
			return false
		}
	}

	if min_answers == 0 && max_answers == 0 && time == 0 {
		return false
	}

	// make unambuguous rules
	if time == 0 {
		if min_answers == 0 {
			min_answers = max_answers
		} else if max_answers == 0 {
			max_answers = min_answers
		} else if min_answers > max_answers {
			min_answers = max_answers
		} else {
			max_answers = min_answers
		}
	} else {
		if (min_answers >= max_answers) {
			min_answers = 0
		}
	}

	db.SetQuestionRules(questionId, min_answers, max_answers, time)
	return true
}

func sendQuestion(bot *tgbotapi.BotAPI, db *database.Database, questionId int64, usersChatIds []int64) {
	var buffer bytes.Buffer

	buffer.WriteString(db.GetQuestionText(questionId) + "\n")

	variants := db.GetQuestionVariants(questionId)
	for i, variant := range(variants) {
		buffer.WriteString(fmt.Sprintf("/ans%d - %s\n", i, variant))
	}

	buffer.WriteString("/skip")
	message := buffer.String()

	for _, chatId := range(usersChatIds) {
		sendMessage(bot, chatId, message)
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
	variants := db.GetQuestionVariants(questionId)
	answers := db.GetQuestionAnswers(questionId)
	answersCount := db.GetQuestionAnswersCount(questionId)

	var buffer bytes.Buffer
	buffer.WriteString(db.GetQuestionText(questionId))

	for i, variant := range(variants) {
		buffer.WriteString(fmt.Sprintf("\n%s - %d (%d%%)", variant, answers[i], int64(100.0*float32(answers[i])/float32(answersCount))))
	}
	resultText := buffer.String()

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
		sendMessage(bot, chatId, T("warn_wrong_answer"))
		return
	}

	if answer >= 0 && int(answer) < variantsCount {
		db.AddQuestionAnswer(questionId, userId, answer)
		db.RemoveUserPendingQuestion(userId, questionId)
		sendMessage(bot, chatId, T("say_answer_added"))

		processCompleteness(bot, db, questionId)

		processNextQuestion(bot, db, userId, chatId)
	} else {
		sendMessage(bot, chatId, T("warn_wrong_answer"))
	}
}

func appendCommand(buffer *bytes.Buffer, command string, description string) {
	buffer.WriteString(fmt.Sprintf("\n%s - %s", command, description))
}

func sendEditingGuide(bot *tgbotapi.BotAPI, db *database.Database, userId int64, chatId int64) {
	T, _ := i18n.Tfunc("en-US")
	questionId := db.GetUserEditingQuestion(userId)

	var buffer bytes.Buffer
	buffer.WriteString(T("question_header"))

	buffer.WriteString(T("text_caption"))
	if db.IsQuestionHasText(questionId) {
		buffer.WriteString(fmt.Sprintf("%s", db.GetQuestionText(questionId)))
	} else {
		buffer.WriteString(T("not_set"))
	}

	buffer.WriteString(T("variants_caption"))
	if db.GetQuestionVariantsCount(questionId) > 0 {
		variants := db.GetQuestionVariants(questionId)

		for i, variant := range(variants) {
			buffer.WriteString(fmt.Sprintf("\n<i>%d</i> - %s", i, variant))
		}
	} else {
		buffer.WriteString(T("not_set"))
	}

	buffer.WriteString(T("rules_caption"))
	if db.IsQuestionHasRules(questionId) {
		min_answers, max_answers, time := db.GetQuestionRules(questionId)
		rulesData := map[string]interface{}{
			"Min": min_answers,
			"Max": max_answers,
			"Time": time,
		}
		var rulesTextFormat string

		if time != 0 {
			if min_answers != 0 {
				if max_answers != 0 {
					rulesTextFormat = "rules_full"
				} else {
					rulesTextFormat = "rules_min_timer"
				}
			} else {
				if max_answers != 0 {
					rulesTextFormat = "rules_max_timer"
				} else {
					rulesTextFormat = "rules_timer"
				}
			}
		} else {
			rulesTextFormat = "rules_min"
		}

		buffer.WriteString(T(rulesTextFormat, rulesData))
	} else {
		buffer.WriteString(T("not_set"))
	}

	appendCommand(&buffer, "/set_text", T("editing_commands_text"))
	appendCommand(&buffer, "/set_variants", T("editing_commands_variants"))
	appendCommand(&buffer, "/set_rules", T("editing_commands_rules"))
	if db.IsQuestionReady(questionId) {
		appendCommand(&buffer, "/commit_question", T("editing_commands_commit"))
	}
	appendCommand(&buffer, "/discard_question", T("editing_commands_discard"))
	sendMessage(bot, chatId, buffer.String())
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
				sendEditingGuide(bot, db, userId, chatId)
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
			if db.IsUserEditingQuestion(userId) {
				sendMessage(bot, chatId, T("warn_unknown_command"))
				sendEditingGuide(bot, db, userId, chatId)
			} else {
				if db.IsUserHasPendingQuestions(userId) {
					parseAnswer(bot, db, chatId, userId, message)
				} else {
					sendMessage(bot, chatId, T("warn_unknown_command"))
				}
			}
		}
	} else {
		if userState, ok := userStates[chatId]; ok {
			switch userState {
			case WaitingText:
				if db.IsUserEditingQuestion(userId) {
					questionId := db.GetUserEditingQuestion(userId)
					db.SetQuestionText(questionId, message)
					sendMessage(bot, chatId, T("say_text_is_set"))
					sendEditingGuide(bot, db, userId, chatId)
					delete(userStates, chatId)
				} else {
					sendMessage(bot, chatId, T("warn_unknown_command"))
					delete(userStates, chatId)
				}
			case WaitingVariants:
				if db.IsUserEditingQuestion(userId) {
					questionId := db.GetUserEditingQuestion(userId)
					ok := setVariants(db, questionId, message)
					if ok {
						sendMessage(bot, chatId, T("say_variants_is_set"))
						sendEditingGuide(bot, db, userId, chatId)
						delete(userStates, chatId)
					} else {
						sendMessage(bot, chatId, T("warn_bad_variants"))
					}
				} else {
					sendMessage(bot, chatId, T("warn_unknown_command"))
					delete(userStates, chatId)
				}
			case WaitingRules:
				if db.IsUserEditingQuestion(userId) {
					questionId := db.GetUserEditingQuestion(userId)
					ok := setRules(db, questionId, message)
					if ok {
						sendMessage(bot, chatId, T("say_rules_is_set"))
						sendEditingGuide(bot, db, userId, chatId)
						delete(userStates, chatId)
					} else {
						sendMessage(bot, chatId, T("warn_bad_rules"))
					}
				} else {
					sendMessage(bot, chatId, T("warn_unknown_command"))
					delete(userStates, chatId)
				}
			default:
				sendMessage(bot, chatId, T("warn_unknown_command"))
				delete(userStates, chatId)
			}
		} else {
			sendMessage(bot, chatId, T("warn_unknown_command"))
			if db.IsUserEditingQuestion(userId) {
				sendEditingGuide(bot, db, userId, chatId)
			}
		}
	}
}

