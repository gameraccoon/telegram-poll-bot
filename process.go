package main

import (
	"bytes"
	"fmt"
	"github.com/gameraccoon/telegram-poll-bot/database"
	"github.com/gameraccoon/telegram-poll-bot/dialogFactories"
	"github.com/gameraccoon/telegram-poll-bot/processing"
	//"github.com/gameraccoon/telegram-poll-bot/telegramChat"
	//"github.com/gameraccoon/telegram-poll-bot/dialog"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"strconv"
	"strings"
	"time"
)

type ProcessorFunc func(*processing.ProcessData, *dialogFactories.DialogManager)

type ProcessorFuncMap map[string]ProcessorFunc

type Processors struct {
	Main      ProcessorFuncMap
	Moderator ProcessorFuncMap
}

func sendResults(staticData *processing.StaticProccessStructs, questionId int64, chatIds []int64) {
	variants := staticData.Db.GetQuestionVariants(questionId)
	answers := staticData.Db.GetQuestionAnswers(questionId)
	answersCount := staticData.Db.GetQuestionAnswersCount(questionId)

	var buffer bytes.Buffer
	buffer.WriteString(staticData.Trans("results_header"))
	buffer.WriteString(fmt.Sprintf("<i>%s</i>", staticData.Db.GetQuestionText(questionId)))

	for i, variant := range variants {
		buffer.WriteString(fmt.Sprintf("\n%s - %d (%d%%)", variant, answers[i], int64(100.0*float32(answers[i])/float32(answersCount))))
	}
	resultText := buffer.String()

	for _, chatId := range chatIds {
		staticData.Chat.SendMessage(chatId, resultText)
	}
}

func removeActiveQuestion(staticData *processing.StaticProccessStructs, questionId int64) {
	staticData.Db.FinishQuestion(questionId)

	delete(staticData.Timers, questionId)

	users := staticData.Db.GetUsersAnsweringQuestionNow(questionId)
	for _, user := range users {
		staticData.Db.RemoveUserPendingQuestion(user, questionId)
		chatId := staticData.Db.GetUserChatId(user)
		staticData.Chat.SendMessage(staticData.Db.GetUserChatId(user), staticData.Trans("say_question_outdated"))

		if staticData.Db.IsUserHasPendingQuestions(user) {
			staticData.Chat.SendQuestion(staticData.Db, staticData.Db.GetUserNextQuestion(user), []int64{chatId})
		} else {
			staticData.Db.MarkUserReady(user)
		}
	}

	staticData.Db.RemoveQuestionFromAllUsers(questionId)
}

func completeQuestion(staticData *processing.StaticProccessStructs, questionId int64) {
	removeActiveQuestion(staticData, questionId)
	chatIds := staticData.Db.GetAllUsersChatIds()
	sendResults(staticData, questionId, chatIds)
}

func isQuestionReadyToBeCompleted(staticData *processing.StaticProccessStructs, questionId int64) bool {
	minAnswers, maxAnswers, _ := staticData.Db.GetQuestionRules(questionId)

	answersCount := staticData.Db.GetQuestionAnswersCount(questionId)

	if answersCount >= maxAnswers && maxAnswers > 0 {
		return true
	}

	if _, ok := staticData.Timers[questionId]; !ok {
		if answersCount >= minAnswers {
			return true
		}
	}

	return false
}

func processCompleteness(staticData *processing.StaticProccessStructs, questionId int64) {
	if isQuestionReadyToBeCompleted(staticData, questionId) {
		completeQuestion(staticData, questionId)
	}
}

func getDificientDataForQuestionText(staticData *processing.StaticProccessStructs, questionId int64) string {
	minAnswers, maxAnswers, endTime := staticData.Db.GetQuestionRules(questionId)
	answersCount := staticData.Db.GetQuestionAnswersCount(questionId)

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

	return processing.GetQuestionRulesText(minAnswers, maxAnswers, timeHours, "delta_answers", staticData.Trans)
}

func sendAnswerFeedback(data *processing.ProcessData, questionId int64) {
	if isQuestionReadyToBeCompleted(data.Static, questionId) {
		data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("say_answer_added"))
		return
	}

	data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("say_answer_added")+"\n"+getDificientDataForQuestionText(data.Static, questionId))
}

func parseAnswer(data *processing.ProcessData) {
	questionId := data.Static.Db.GetUserNextQuestion(data.UserId)
	variantsCount := data.Static.Db.GetQuestionVariantsCount(questionId)

	if data.Command == "skip" {
		data.Static.Db.RemoveUserPendingQuestion(data.UserId, questionId)
		data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("say_question_skipped"))

		processCompleteness(data.Static, questionId)

		processing.ProcessNextQuestion(data)
		return
	}

	if !strings.HasPrefix(data.Command, "ans") {
		data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("warn_wrong_answer"))
		return
	}

	answer, err := strconv.ParseInt(data.Command[3:len(data.Command)], 10, 64)
	if err != nil {
		data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("warn_wrong_answer"))
		return
	}

	answer -= 1

	if answer >= 0 && int(answer) < variantsCount {
		data.Static.Db.AddQuestionAnswer(questionId, data.UserId, answer)
		data.Static.Db.RemoveUserPendingQuestion(data.UserId, questionId)

		sendAnswerFeedback(data, questionId)

		processCompleteness(data.Static, questionId)

		processing.ProcessNextQuestion(data)
	} else {
		data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("warn_wrong_answer"))
	}
}

func sendEditingGuide(data *processing.ProcessData, dialogManager *dialogFactories.DialogManager) {
	dialog := dialogManager.MakeDialog("ed", data)
	if dialog != nil {
		data.Static.Chat.SendDialog(dialog, data.ChatId)
	}
}

func addQuestionCommand(data *processing.ProcessData, dialogManager *dialogFactories.DialogManager) {
	if data.Static.Db.IsUserBanned(data.UserId) {
		data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("warn_youre_banned"))
		return
	}
	if !data.Static.Db.IsUserEditingQuestion(data.UserId) {
		data.Static.Db.StartCreatingQuestion(data.UserId)
		data.Static.Db.UnmarkUserReady(data.UserId)
		data.Static.UserStates[data.ChatId] = processing.WaitingText
		data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("ask_question_text"))
	} else {
		sendEditingGuide(data, dialogManager)
	}
}

func startCommand(data *processing.ProcessData, dialogManager *dialogFactories.DialogManager) {
	data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("hello_message"))
	if !data.Static.Db.IsUserHasPendingQuestions(data.UserId) {
		data.Static.Db.InitNewUserQuestions(data.UserId)
		data.Static.Db.UnmarkUserReady(data.UserId)
		processing.ProcessNextQuestion(data)
	}
}

func lastResultsCommand(data *processing.ProcessData, dialogManager *dialogFactories.DialogManager) {
	questions := data.Static.Db.GetLastFinishedQuestions(10)
	for _, questionId := range questions {
		sendResults(data.Static, questionId, []int64{data.ChatId})
	}
}

func myQuestionsCommand(data *processing.ProcessData, dialogManager *dialogFactories.DialogManager) {
	questionsIds := data.Static.Db.GetUserLastQuestions(data.UserId, 10)
	finishedQuestionsIds := data.Static.Db.GetUserLastFinishedQuestions(data.UserId, 10)

	finishedQuestionsMap := make(map[int64]bool)
	for _, questionId := range finishedQuestionsIds {
		finishedQuestionsMap[questionId] = true
	}

	for _, questionId := range questionsIds {
		if _, ok := finishedQuestionsMap[questionId]; ok {
			sendResults(data.Static, questionId, []int64{data.ChatId})
		} else {
			data.Static.Chat.SendMessage(data.ChatId, fmt.Sprintf("<i>%s</i>\n%s",
				data.Static.Db.GetQuestionText(questionId),
				getDificientDataForQuestionText(data.Static, questionId),
			))
		}
	}
}

func moderatorListCommand(data *processing.ProcessData, dialogManager *dialogFactories.DialogManager) {
	questions := data.Static.Db.GetLastPublishedQuestions(15)
	var buffer bytes.Buffer
	for _, question := range questions {
		buffer.WriteString(fmt.Sprintf("%d - %s\n", question, data.Static.Db.GetQuestionText(question)))
	}
	data.Static.Chat.SendMessage(data.ChatId, buffer.String())
}

func moderatorBanCommand(data *processing.ProcessData, dialogManager *dialogFactories.DialogManager) {
	questionId, err := strconv.ParseInt(data.Message, 10, 64)

	if err != nil {
		return
	}

	author, err := data.Static.Db.GetAuthor(questionId)
	if err != nil {
		return
	}
	data.Static.Db.BanUser(author)
	data.Static.Chat.SendMessage(data.ChatId, fmt.Sprintf("banned: %d", author))
}

func moderatorRemoveCommand(data *processing.ProcessData, dialogManager *dialogFactories.DialogManager) {
	questionId, err := strconv.ParseInt(data.Message, 10, 64)

	if err != nil {
		return
	}

	removeActiveQuestion(data.Static, questionId)
	data.Static.Db.RemoveQuestion(questionId)
	data.Static.Chat.SendMessage(data.ChatId, "removed")
}

func moderatorSendCommand(data *processing.ProcessData, dialogManager *dialogFactories.DialogManager) {
	chatIds := data.Static.Db.GetAllUsersChatIds()
	for _, chatId := range chatIds {
		data.Static.Chat.SendMessage(chatId, data.Message)
	}
}

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
		if minAnswers >= maxAnswers && maxAnswers > 0 {
			time = 0
			minAnswers = maxAnswers
		}
	}

	db.SetQuestionRules(questionId, minAnswers, maxAnswers, time)
	return true
}

func makeUserCommandProcessors() ProcessorFuncMap {
	return map[string]ProcessorFunc{
		"start":        startCommand,
		"add_question": addQuestionCommand,
		"last_results": lastResultsCommand,
		"my_questions": myQuestionsCommand,
	}
}

func makeModeratorCommandProcessors() ProcessorFuncMap {
	return map[string]ProcessorFunc{
		"m_list": moderatorListCommand,
		"m_ban":  moderatorBanCommand,
		"m_rm":   moderatorRemoveCommand,
		"m_send": moderatorSendCommand,
	}
}

func processAnswer(data *processing.ProcessData) bool {
	if data.Static.Db.IsUserHasPendingQuestions(data.UserId) {
		parseAnswer(data)
		return true
	}
	return false
}

func processCommandByProcessors(data *processing.ProcessData, processorsMap ProcessorFuncMap, dialogManager *dialogFactories.DialogManager) bool {
	processor, ok := processorsMap[data.Command]
	if ok {
		processor(data, dialogManager)
	}

	return ok
}

func processCommand(data *processing.ProcessData, dialogManager *dialogFactories.DialogManager, processors *Processors) {
	if strings.HasPrefix(data.Command, "m_") && isUserModerator(data.ChatId, data.Static.Config) {
		processed := processCommandByProcessors(data, processors.Moderator, dialogManager)
		if processed {
			return
		}
	}

	ids := strings.Split(data.Command, "_")
	if len(ids) >= 2 {
		processed := dialogManager.ProcessVariant(ids[0], ids[1], data)
		if processed {
			return
		}
	}

	processed := processCommandByProcessors(data, processors.Main, dialogManager)
	if processed {
		return
	}

	isEditingQuestion := data.Static.Db.IsUserEditingQuestion(data.UserId)
	if !isEditingQuestion {
		processed = processAnswer(data)
		if processed {
			return
		}
	}

	// if we here it means that no command was processed
	data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("warn_unknown_command"))
	if isEditingQuestion {
		sendEditingGuide(data, dialogManager)
	}
}

func isUserModerator(chatId int64, config *processing.StaticConfiguration) bool {
	for _, moderator := range config.Moderators {
		if chatId == moderator {
			return true
		}
	}

	return false
}

func processSetTextContent(data *processing.ProcessData, dialogManager *dialogFactories.DialogManager) {
	if data.Static.Db.IsUserEditingQuestion(data.UserId) {
		questionId := data.Static.Db.GetUserEditingQuestion(data.UserId)
		data.Static.Db.SetQuestionText(questionId, data.Message)
		data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("say_text_is_set"))
		sendEditingGuide(data, dialogManager)
		delete(data.Static.UserStates, data.ChatId)
	} else {
		data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("warn_unknown_command"))
		delete(data.Static.UserStates, data.ChatId)
	}
}

func processSetVariantsContent(data *processing.ProcessData, dialogManager *dialogFactories.DialogManager) {
	if data.Static.Db.IsUserEditingQuestion(data.UserId) {
		questionId := data.Static.Db.GetUserEditingQuestion(data.UserId)
		ok := setVariants(data.Static.Db, questionId, &data.Message)
		if ok {
			data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("say_variants_is_set"))
			sendEditingGuide(data, dialogManager)
			delete(data.Static.UserStates, data.ChatId)
		} else {
			data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("warn_bad_variants"))
		}
	} else {
		data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("warn_unknown_command"))
		delete(data.Static.UserStates, data.ChatId)
	}
}

func processSetRulesContent(data *processing.ProcessData, dialogManager *dialogFactories.DialogManager) {
	if data.Static.Db.IsUserEditingQuestion(data.UserId) {
		questionId := data.Static.Db.GetUserEditingQuestion(data.UserId)
		ok := setRules(data.Static.Db, questionId, &data.Message)
		if ok {
			data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("say_rules_is_set"))
			sendEditingGuide(data, dialogManager)
			delete(data.Static.UserStates, data.ChatId)
		} else {
			data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("warn_bad_rules"))
		}
	} else {
		data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("warn_unknown_command"))
		delete(data.Static.UserStates, data.ChatId)
	}
}

func processPlainMessage(data *processing.ProcessData, dialogManager *dialogFactories.DialogManager) {
	if userState, ok := data.Static.UserStates[data.ChatId]; ok {
		switch userState {
		case processing.WaitingText:
			processSetTextContent(data, dialogManager)
		case processing.WaitingVariants:
			processSetVariantsContent(data, dialogManager)
		case processing.WaitingRules:
			processSetRulesContent(data, dialogManager)
		default:
			data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("warn_unknown_command"))
			delete(data.Static.UserStates, data.ChatId)
		}
	} else {
		data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("warn_unknown_command"))
		if data.Static.Db.IsUserEditingQuestion(data.UserId) {
			sendEditingGuide(data, dialogManager)
		}
	}
}

func processUpdate(update *tgbotapi.Update, staticData *processing.StaticProccessStructs, dialogManager *dialogFactories.DialogManager, processors *Processors) {
	data := processing.ProcessData{
		Static: staticData,
		ChatId: update.Message.Chat.ID,
		UserId: staticData.Db.GetUserId(update.Message.Chat.ID),
	}

	message := update.Message.Text

	if strings.HasPrefix(message, "/") {
		commandLen := strings.Index(message, " ")
		if commandLen != -1 {
			data.Command = message[1:commandLen]
			data.Message = message[commandLen+1:]
		} else {
			data.Command = message[1:]
		}

		processCommand(&data, dialogManager, processors)
	} else {
		data.Message = message
		processPlainMessage(&data, dialogManager)
	}
}

func processTimer(staticData *processing.StaticProccessStructs, questionId int64) {
	processCompleteness(staticData, questionId)
}
