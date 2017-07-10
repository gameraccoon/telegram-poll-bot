package processing

import (
	"bytes"
	"fmt"
	"github.com/gameraccoon/telegram-poll-bot/database"
	"github.com/gameraccoon/telegram-poll-bot/chat/telegram"
	"github.com/nicksnyder/go-i18n/i18n"
	"time"
)

func ProcessNextQuestion(data *ProcessData) {
	if data.Static.Db.IsUserHasPendingQuestions(data.UserId) {
		nextQuestion := data.Static.Db.GetUserNextQuestion(data.UserId)
		telegram.SendQuestion(data.Static.Bot, data.Static.Db, nextQuestion, []int64{data.ChatId})
	} else {
		data.Static.Db.MarkUserReady(data.UserId)
	}
}

func CommitQuestion(data *ProcessData, questionId int64) {
	data.Static.Db.CommitQuestion(questionId)
	telegram.SendMessage(data.Static.Bot, data.ChatId, data.Static.Trans("say_question_commited"))

	minAnswers, maxAnswers, durationTime := data.Static.Db.GetQuestionRules(questionId)

	endTime := time.Now().Add(time.Duration(durationTime) * time.Hour)
	data.Static.Timers[questionId] = endTime

	data.Static.Db.SetQuestionRules(questionId, minAnswers, maxAnswers, endTime.Unix())

	ProcessNextQuestion(data)

	users := data.Static.Db.GetReadyUsersChatIds()

	telegram.SendQuestion(data.Static.Bot, data.Static.Db, questionId, users)
}

func GetQuestionRulesText(minAnswers int, maxAnswers int, time int64, answersTag string, trans i18n.TranslateFunc) string {
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
