package main

import (
	"bytes"
	"fmt"
	"github.com/gameraccoon/telegram-poll-bot/database"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/nicksnyder/go-i18n/i18n"
	"strconv"
	"strings"
	"time"
)

func setVariants(db *database.Database, questionId int64, message *string) (ok bool) {
	variants := strings.Split(*message, "\n")
	db.SetQuestionVariants(questionId, variants)
	return true
}

func setRules(db *database.Database, questionId int64, message *string) (ok bool) {
	rules := strings.Split(*message, " ")
	if len(rules) == 0 {
		return false
	}

	var minAnswers int
	var maxAnswers int
	var time int64
	var err error

	if len(rules) >= 1 {
		minAnswers, err = strconv.Atoi(rules[0])
		if err != nil || minAnswers < 0 {
			return false
		}
	}

	if len(rules) >= 2 {
		maxAnswers, err = strconv.Atoi(rules[1])
		if err != nil || maxAnswers < 0 {
			return false
		}
	}

	if len(rules) >= 3 {
		time, err = strconv.ParseInt(rules[2], 10, 64)
		if err != nil || time < 0 {
			return false
		}
	}

	if minAnswers == 0 && maxAnswers == 0 && time == 0 {
		return false
	}

	// make unambuguous rules
	if time == 0 {
		if minAnswers == 0 {
			minAnswers = maxAnswers
		} else if maxAnswers == 0 {
			maxAnswers = minAnswers
		} else if minAnswers > maxAnswers {
			minAnswers = maxAnswers
		} else {
			maxAnswers = minAnswers
		}
	} else {
		if minAnswers >= maxAnswers {
			time = 0
			minAnswers = maxAnswers
		}
	}

	db.SetQuestionRules(questionId, minAnswers, maxAnswers, time)
	return true
}

func sendQuestion(bot *tgbotapi.BotAPI, db *database.Database, questionId int64, usersChatIds []int64) {
	var buffer bytes.Buffer

	buffer.WriteString(db.GetQuestionText(questionId) + "\n")

	variants := db.GetQuestionVariants(questionId)
	for i, variant := range variants {
		buffer.WriteString(fmt.Sprintf("/ans%d - %s\n", i+1, variant))
	}

	buffer.WriteString("/skip")
	message := buffer.String()

	for _, chatId := range usersChatIds {
		sendMessage(bot, chatId, message)
	}

	db.UnmarkUsersReady(usersChatIds)
}

func processNextQuestion(data *processData) {
	if data.static.db.IsUserHasPendingQuestions(data.userId) {
		nextQuestion := data.static.db.GetUserNextQuestion(data.userId)
		sendQuestion(data.static.bot, data.static.db, nextQuestion, []int64{data.chatId})
	} else {
		data.static.db.MarkUserReady(data.userId)
	}
}

func commitQuestion(data *processData, questionId int64) {
	data.static.db.CommitQuestion(questionId)
	sendMessage(data.static.bot, data.chatId, data.static.trans("say_question_commited"))

	minAnswers, maxAnswers, durationTime := data.static.db.GetQuestionRules(questionId)

	endTime := time.Now().Add(time.Duration(durationTime) * time.Hour)
	data.static.timers[questionId] = endTime

	data.static.db.SetQuestionRules(questionId, minAnswers, maxAnswers, endTime.Unix())

	processNextQuestion(data)

	users := data.static.db.GetReadyUsersChatIds()

	sendQuestion(data.static.bot, data.static.db, questionId, users)
}

func sendResults(staticData *staticProccessStructs, questionId int64, chatIds []int64) {
	variants := staticData.db.GetQuestionVariants(questionId)
	answers := staticData.db.GetQuestionAnswers(questionId)
	answersCount := staticData.db.GetQuestionAnswersCount(questionId)

	var buffer bytes.Buffer
	buffer.WriteString(staticData.trans("results_header"))
	buffer.WriteString(fmt.Sprintf("<i>%s</i>", staticData.db.GetQuestionText(questionId)))

	for i, variant := range variants {
		buffer.WriteString(fmt.Sprintf("\n%s - %d (%d%%)", variant, answers[i], int64(100.0*float32(answers[i])/float32(answersCount))))
	}
	resultText := buffer.String()

	for _, chatId := range chatIds {
		sendMessage(staticData.bot, chatId, resultText)
	}
}

func removeActiveQuestion(staticData *staticProccessStructs, questionId int64) {
	staticData.db.EndQuestion(questionId)

	delete(staticData.timers, questionId)

	users := staticData.db.GetUsersAnsweringQuestionNow(questionId)
	for _, user := range users {
		staticData.db.RemoveUserPendingQuestion(user, questionId)
		chatId := staticData.db.GetUserChatId(user)
		sendMessage(staticData.bot, staticData.db.GetUserChatId(user), staticData.trans("say_question_outdated"))

		if staticData.db.IsUserHasPendingQuestions(user) {
			sendQuestion(staticData.bot, staticData.db, staticData.db.GetUserNextQuestion(user), []int64{chatId})
		} else {
			staticData.db.MarkUserReady(user)
		}
	}

	staticData.db.RemoveQuestionFromAllUsers(questionId)
}

func completeQuestion(staticData *staticProccessStructs, questionId int64) {
	removeActiveQuestion(staticData, questionId)
	chatIds := staticData.db.GetAllUsersChatIds()
	sendResults(staticData, questionId, chatIds)
}

func isQuestionReadyToBeCompleted(staticData *staticProccessStructs, questionId int64) bool {
	minAnswers, maxAnswers, _ := staticData.db.GetQuestionRules(questionId)

	answersCount := staticData.db.GetQuestionAnswersCount(questionId)

	if answersCount >= maxAnswers && maxAnswers > 0 {
		return true
	}

	if _, ok := staticData.timers[questionId]; !ok {
		if answersCount >= minAnswers {
			return true
		}
	}

	return false
}

func processCompleteness(staticData *staticProccessStructs, questionId int64) {
	if isQuestionReadyToBeCompleted(staticData, questionId) {
		completeQuestion(staticData, questionId)
	}
}

func sendAnswerFeedback(data *processData, questionId int64) {
	if isQuestionReadyToBeCompleted(data.static, questionId) {
		sendMessage(data.static.bot, data.chatId, data.static.trans("say_answer_added"))
		return
	}

	minAnswers, maxAnswers, endTime := data.static.db.GetQuestionRules(questionId)
	answersCount := data.static.db.GetQuestionAnswersCount(questionId)

	// recalculate currently deficient values
	minAnswers = minAnswers - answersCount
	maxAnswers = maxAnswers - answersCount
	var timeHours int64
	if endTime > 0 {
		timeHours = int64(time.Unix(endTime, 0).Sub(time.Now()).Hours() + 1)
	}

	if minAnswers < 0 {
		minAnswers = 0
	}

	if maxAnswers < 0 {
		maxAnswers = 0
	}

	if timeHours < 0 {
		timeHours = 0
	}

	sendMessage(data.static.bot, data.chatId, getQuestionRulesText(minAnswers, maxAnswers, timeHours, "delta_answers", data.static.trans))
}

func parseAnswer(data *processData) {
	questionId := data.static.db.GetUserNextQuestion(data.userId)
	variantsCount := data.static.db.GetQuestionVariantsCount(questionId)

	if data.command == "skip" {
		data.static.db.RemoveUserPendingQuestion(data.userId, questionId)
		sendMessage(data.static.bot, data.chatId, data.static.trans("say_question_skipped"))

		processCompleteness(data.static, questionId)

		processNextQuestion(data)
		return
	}

	if !strings.HasPrefix(data.command, "ans") {
		sendMessage(data.static.bot, data.chatId, data.static.trans("warn_wrong_answer"))
		return
	}

	answer, err := strconv.ParseInt(data.command[3:len(data.command)], 10, 64)
	if err != nil {
		sendMessage(data.static.bot, data.chatId, data.static.trans("warn_wrong_answer"))
		return
	}

	answer -= 1

	if answer >= 0 && int(answer) < variantsCount {
		data.static.db.AddQuestionAnswer(questionId, data.userId, answer)
		data.static.db.RemoveUserPendingQuestion(data.userId, questionId)

		sendAnswerFeedback(data, questionId)

		processCompleteness(data.static, questionId)

		processNextQuestion(data)
	} else {
		sendMessage(data.static.bot, data.chatId, data.static.trans("warn_wrong_answer"))
	}
}

func appendCommand(buffer *bytes.Buffer, command string, description string) {
	buffer.WriteString(fmt.Sprintf("\n%s - %s", command, description))
}

func getQuestionRulesText(minAnswers int, maxAnswers int, time int64, answersTag string, trans i18n.TranslateFunc) string {
	rulesData := map[string]interface{}{
		"Min":  trans(answersTag, minAnswers),
		"Max":  trans(answersTag, maxAnswers),
		"Time": trans("hours", time),
	}
	var rulesTextFormat string

	if time != 0 {
		if minAnswers != 0 {
			if maxAnswers != 0 {
				rulesTextFormat = "rules_full"
			} else {
				rulesTextFormat = "rules_min_timer"
			}
		} else {
			if maxAnswers != 0 {
				rulesTextFormat = "rules_max_timer"
			} else {
				rulesTextFormat = "rules_timer"
			}
		}
	} else {
		rulesTextFormat = "rules_min"
	}

	return trans(rulesTextFormat, rulesData)

}

func sendEditingGuide(data *processData) {
	questionId := data.static.db.GetUserEditingQuestion(data.userId)

	var buffer bytes.Buffer
	buffer.WriteString(data.static.trans("question_header"))

	buffer.WriteString(data.static.trans("text_caption"))
	if data.static.db.IsQuestionHasText(questionId) {
		buffer.WriteString(fmt.Sprintf("%s", data.static.db.GetQuestionText(questionId)))
	} else {
		buffer.WriteString(data.static.trans("not_set"))
	}

	buffer.WriteString(data.static.trans("variants_caption"))
	if data.static.db.GetQuestionVariantsCount(questionId) > 0 {
		variants := data.static.db.GetQuestionVariants(questionId)

		for i, variant := range variants {
			buffer.WriteString(fmt.Sprintf("\n<i>%d</i> - %s", i+1, variant))
		}
	} else {
		buffer.WriteString(data.static.trans("not_set"))
	}

	buffer.WriteString(data.static.trans("rules_caption"))
	if data.static.db.IsQuestionHasRules(questionId) {
		minAnswers, maxAnswers, time := data.static.db.GetQuestionRules(questionId)
		buffer.WriteString(getQuestionRulesText(minAnswers, maxAnswers, time, "answers", data.static.trans))
	} else {
		buffer.WriteString(data.static.trans("not_set"))
	}

	appendCommand(&buffer, "/set_text", data.static.trans("editing_commands_text"))
	appendCommand(&buffer, "/set_variants", data.static.trans("editing_commands_variants"))
	appendCommand(&buffer, "/set_rules", data.static.trans("editing_commands_rules"))
	if data.static.db.IsQuestionReady(questionId) {
		appendCommand(&buffer, "/commit_question", data.static.trans("editing_commands_commit"))
	}
	appendCommand(&buffer, "/discard_question", data.static.trans("editing_commands_discard"))
	sendMessage(data.static.bot, data.chatId, buffer.String())
}

func addQuestionCommand(data *processData) {
	if data.static.db.IsUserBanned(data.userId) {
		sendMessage(data.static.bot, data.chatId, data.static.trans("warn_youre_banned"))
		return
	}
	if !data.static.db.IsUserEditingQuestion(data.userId) {
		data.static.db.StartCreatingQuestion(data.userId)
		data.static.db.UnmarkUserReady(data.userId)
		data.static.userStates[data.chatId] = WaitingText
		sendMessage(data.static.bot, data.chatId, data.static.trans("ask_question_text"))
	} else {
		sendEditingGuide(data)
	}
}

func setTextCommand(data *processData) {
	if data.static.db.IsUserEditingQuestion(data.userId) {
		data.static.userStates[data.chatId] = WaitingText
		sendMessage(data.static.bot, data.chatId, data.static.trans("ask_question_text"))
	} else {
		sendMessage(data.static.bot, data.chatId, data.static.trans("warn_not_editing_question"))
	}
}

func setVariantsCommand(data *processData) {
	if data.static.db.IsUserEditingQuestion(data.userId) {
		data.static.userStates[data.chatId] = WaitingVariants
		sendMessage(data.static.bot, data.chatId, data.static.trans("ask_variants"))
	} else {
		sendMessage(data.static.bot, data.chatId, data.static.trans("warn_not_editing_question"))
	}
}

func setRulesCommand(data *processData) {
	if data.static.db.IsUserEditingQuestion(data.userId) {
		data.static.userStates[data.chatId] = WaitingRules
		sendMessage(data.static.bot, data.chatId, data.static.trans("ask_rules"))
	} else {
		sendMessage(data.static.bot, data.chatId, data.static.trans("warn_not_editing_question"))
	}
}

func commitQuestionCommand(data *processData) {
	if data.static.db.IsUserBanned(data.userId) {
		sendMessage(data.static.bot, data.chatId, data.static.trans("warn_youre_banned"))
		if data.static.db.IsUserEditingQuestion(data.userId) {
			questionId := data.static.db.GetUserEditingQuestion(data.userId)
			data.static.db.DiscardQuestion(questionId)
			processNextQuestion(data)
		}
		return
	}
	if data.static.db.IsUserEditingQuestion(data.userId) {
		questionId := data.static.db.GetUserEditingQuestion(data.userId)
		if data.static.db.IsQuestionReady(questionId) && data.static.db.GetQuestionVariantsCount(questionId) > 0 {
			commitQuestion(data, questionId)
		} else {
			sendMessage(data.static.bot, data.chatId, data.static.trans("warn_question_not_ready"))
		}
	} else {
		sendMessage(data.static.bot, data.chatId, data.static.trans("warn_not_editing_question"))
	}
}

func discardQuestionCommand(data *processData) {
	if data.static.db.IsUserEditingQuestion(data.userId) {
		questionId := data.static.db.GetUserEditingQuestion(data.userId)
		data.static.db.DiscardQuestion(questionId)
		sendMessage(data.static.bot, data.chatId, data.static.trans("say_question_discarded"))
		processNextQuestion(data)
	} else {
		sendMessage(data.static.bot, data.chatId, data.static.trans("warn_not_editing_question"))
	}
}

func startCommand(data *processData) {
	sendMessage(data.static.bot, data.chatId, data.static.trans("hello_message"))
	if !data.static.db.IsUserHasPendingQuestions(data.userId) {
		data.static.db.InitNewUserQuestions(data.userId)
		data.static.db.UnmarkUserReady(data.userId)
		processNextQuestion(data)
	}
}

func lastResultsCommand(data *processData) {
	questions := data.static.db.GetLastFinishedQuestions(10)
	for _, questionId := range questions {
		sendResults(data.static, questionId, []int64{data.chatId})
	}
}

func moderatorListCommand(data *processData) {
	questions := data.static.db.GetLastPublishedQuestions(15)
	var buffer bytes.Buffer
	for _, question := range questions {
		buffer.WriteString(fmt.Sprintf("%d - %s\n", question, data.static.db.GetQuestionText(question)))
	}
	sendMessage(data.static.bot, data.chatId, buffer.String())
}

func moderatorBanCommand(data *processData) {
	questionId, err := strconv.ParseInt(data.message, 10, 64)

	if err != nil {
		return
	}

	author, err := data.static.db.GetAuthor(questionId)
	if err != nil {
		return
	}
	data.static.db.BanUser(author)
	sendMessage(data.static.bot, data.chatId, fmt.Sprintf("banned: %d", author))
}

func moderatorRemoveCommand(data *processData) {
	questionId, err := strconv.ParseInt(data.message, 10, 64)

	if err != nil {
		return
	}

	removeActiveQuestion(data.static, questionId)
	data.static.db.RemoveQuestion(questionId)
	sendMessage(data.static.bot, data.chatId, "removed")
}

func moderatorSendCommand(data *processData) {
	sendMessage(data.static.bot, data.chatId, data.message)
}

type staticProccessStructs struct {
	bot                 *tgbotapi.BotAPI
	db                  *database.Database
	userStates          map[int64]userState
	timers              map[int64]time.Time
	config              *configuration
	trans               i18n.TranslateFunc
	processors          map[string]func(*processData)
	moderatorProcessors map[string]func(*processData)
}

type processData struct {
	static  *staticProccessStructs
	command string // first part of command without /
	message string // parameters of command or plain message
	chatId  int64
	userId  int64
}

func makeUserCommandProcessors() map[string]func(*processData) {
	return map[string]func(*processData){
		"start":            startCommand,
		"add_question":     addQuestionCommand,
		"set_text":         setTextCommand,
		"set_variants":     setVariantsCommand,
		"set_rules":        setRulesCommand,
		"commit_question":  commitQuestionCommand,
		"discard_question": discardQuestionCommand,
		"last_results":     lastResultsCommand,
	}
}

func makeModeratorCommandProcessors() map[string]func(*processData) {
	return map[string]func(*processData){
		"m_list": moderatorListCommand,
		"m_ban":  moderatorBanCommand,
		"m_rm":   moderatorRemoveCommand,
		"m_send": moderatorSendCommand,
	}
}

func processAnswer(data *processData) bool {
	if data.static.db.IsUserHasPendingQuestions(data.userId) {
		parseAnswer(data)
		return true
	}
	return false
}

func processCommandByProcessors(data *processData, processors map[string]func(*processData)) bool {
	processor, ok := processors[data.command]
	if ok {
		processor(data)
	}

	return ok
}

func processCommand(data *processData) {
	if strings.HasPrefix(data.command, "m_") && isUserModerator(data.chatId, data.static.config) {
		processed := processCommandByProcessors(data, data.static.moderatorProcessors)
		if processed {
			return
		}
	}

	processed := processCommandByProcessors(data, data.static.processors)
	if processed {
		return
	}

	isEditingQuestion := data.static.db.IsUserEditingQuestion(data.userId)
	if !isEditingQuestion {
		processed = processAnswer(data)
		if processed {
			return
		}
	}

	// if we here it means that no command was processed
	sendMessage(data.static.bot, data.chatId, data.static.trans("warn_unknown_command"))
	if isEditingQuestion {
		sendEditingGuide(data)
	}
}

func isUserModerator(chatId int64, config *configuration) bool {
	for _, moderator := range config.Moderators {
		if chatId == moderator {
			return true
		}
	}

	return false
}

func processSetTextContent(data *processData) {
	if data.static.db.IsUserEditingQuestion(data.userId) {
		questionId := data.static.db.GetUserEditingQuestion(data.userId)
		data.static.db.SetQuestionText(questionId, data.message)
		sendMessage(data.static.bot, data.chatId, data.static.trans("say_text_is_set"))
		sendEditingGuide(data)
		delete(data.static.userStates, data.chatId)
	} else {
		sendMessage(data.static.bot, data.chatId, data.static.trans("warn_unknown_command"))
		delete(data.static.userStates, data.chatId)
	}
}

func processSetVariantsContent(data *processData) {
	if data.static.db.IsUserEditingQuestion(data.userId) {
		questionId := data.static.db.GetUserEditingQuestion(data.userId)
		ok := setVariants(data.static.db, questionId, &data.message)
		if ok {
			sendMessage(data.static.bot, data.chatId, data.static.trans("say_variants_is_set"))
			sendEditingGuide(data)
			delete(data.static.userStates, data.chatId)
		} else {
			sendMessage(data.static.bot, data.chatId, data.static.trans("warn_bad_variants"))
		}
	} else {
		sendMessage(data.static.bot, data.chatId, data.static.trans("warn_unknown_command"))
		delete(data.static.userStates, data.chatId)
	}
}

func processSetRulesContent(data *processData) {
	if data.static.db.IsUserEditingQuestion(data.userId) {
		questionId := data.static.db.GetUserEditingQuestion(data.userId)
		ok := setRules(data.static.db, questionId, &data.message)
		if ok {
			sendMessage(data.static.bot, data.chatId, data.static.trans("say_rules_is_set"))
			sendEditingGuide(data)
			delete(data.static.userStates, data.chatId)
		} else {
			sendMessage(data.static.bot, data.chatId, data.static.trans("warn_bad_rules"))
		}
	} else {
		sendMessage(data.static.bot, data.chatId, data.static.trans("warn_unknown_command"))
		delete(data.static.userStates, data.chatId)
	}
}

func processPlainMessage(data *processData) {
	if userState, ok := data.static.userStates[data.chatId]; ok {
		switch userState {
		case WaitingText:
			processSetTextContent(data)
		case WaitingVariants:
			processSetVariantsContent(data)
		case WaitingRules:
			processSetRulesContent(data)
		default:
			sendMessage(data.static.bot, data.chatId, data.static.trans("warn_unknown_command"))
			delete(data.static.userStates, data.chatId)
		}
	} else {
		sendMessage(data.static.bot, data.chatId, data.static.trans("warn_unknown_command"))
		if data.static.db.IsUserEditingQuestion(data.userId) {
			sendEditingGuide(data)
		}
	}
}

func processUpdate(update *tgbotapi.Update, staticData *staticProccessStructs) {
	data := processData{
		static: staticData,
		chatId: update.Message.Chat.ID,
		userId: staticData.db.GetUserId(update.Message.Chat.ID),
	}

	message := update.Message.Text

	if strings.HasPrefix(message, "/") {
		commandLen := strings.Index(message, " ")
		if commandLen != -1 {
			data.command = message[1:commandLen]
			data.message = message[commandLen+1:]
		} else {
			data.command = message[1:]
		}

		processCommand(&data)
	} else {
		data.message = message
		processPlainMessage(&data)
	}
}

func processTimer(staticData *staticProccessStructs, questionId int64) {
	processCompleteness(staticData, questionId)
}
