package dialogFactories

import (
	"bytes"
	"fmt"
	"github.com/gameraccoon/telegram-poll-bot/processing"
)

func GetQuestionEditDialog() Dialog {
	return Dialog {
		text : "",
		getText : getEditingGuide,
		variants : map[string]Variant {
			"st" : Variant {
				name : "editing_commands_text",
				isActive : nil,
				process : setTextCommand,
			},
			"sv" : Variant {
				name : "editing_commands_variants",
				isActive : nil,
				process : setVariantsCommand,
			},
			"sr" : Variant {
				name : "editing_commands_rules",
				isActive : nil,
				process : setRulesCommand,
			},
			"co" : Variant {
				name : "editing_commands_commit",
				isActive : func(data *processing.ProcessData) bool {
					questionId := data.Static.Db.GetUserEditingQuestion(data.UserId)
					return data.Static.Db.IsQuestionReady(questionId)
				},
				process : commitQuestionCommand,
			},
			"qi" : Variant {
				name : "editing_commands_discard",
				isActive : nil,
				process : discardQuestionCommand,
			},
		},
	}
}

func setTextCommand(data *processing.ProcessData) {
	if data.Static.Db.IsUserEditingQuestion(data.UserId) {
		data.Static.UserStates[data.ChatId] = processing.WaitingText
		data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("ask_question_text"))
	} else {
		data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("warn_not_editing_question"))
	}
}

func setVariantsCommand(data *processing.ProcessData) {
	if data.Static.Db.IsUserEditingQuestion(data.UserId) {
		data.Static.UserStates[data.ChatId] = processing.WaitingVariants
		data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("ask_variants"))
	} else {
		data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("warn_not_editing_question"))
	}
}

func setRulesCommand(data *processing.ProcessData) {
	if data.Static.Db.IsUserEditingQuestion(data.UserId) {
		data.Static.UserStates[data.ChatId] = processing.WaitingRules
		data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("ask_rules"))
	} else {
		data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("warn_not_editing_question"))
	}
}

func commitQuestionCommand(data *processing.ProcessData) {
	if data.Static.Db.IsUserBanned(data.UserId) {
		data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("warn_youre_banned"))
		if data.Static.Db.IsUserEditingQuestion(data.UserId) {
			questionId := data.Static.Db.GetUserEditingQuestion(data.UserId)
			data.Static.Db.DiscardQuestion(questionId)
			processing.ProcessNextQuestion(data)
		}
		return
	}
	if data.Static.Db.IsUserEditingQuestion(data.UserId) {
		questionId := data.Static.Db.GetUserEditingQuestion(data.UserId)
		if data.Static.Db.IsQuestionReady(questionId) && data.Static.Db.GetQuestionVariantsCount(questionId) > 0 {
			processing.CommitQuestion(data, questionId)
		} else {
			data.Static.Chat.SendMessage(ddata.ChatId, data.Static.Trans("warn_question_not_ready"))
		}
	} else {
		data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("warn_not_editing_question"))
	}
}

func discardQuestionCommand(data *processing.ProcessData) {
	if data.Static.Db.IsUserEditingQuestion(data.UserId) {
		questionId := data.Static.Db.GetUserEditingQuestion(data.UserId)
		data.Static.Db.DiscardQuestion(questionId)
		data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("say_question_discarded"))
		processing.ProcessNextQuestion(data)
	} else {
		data.Static.Chat.SendMessage(data.ChatId, data.Static.Trans("warn_not_editing_question"))
	}
}

func getEditingGuide(data *processing.ProcessData) string {
	questionId := data.Static.Db.GetUserEditingQuestion(data.UserId)

	var buffer bytes.Buffer
	buffer.WriteString(data.Static.Trans("question_header"))

	buffer.WriteString(data.Static.Trans("text_caption"))
	if data.Static.Db.IsQuestionHasText(questionId) {
		buffer.WriteString(fmt.Sprintf("%s", data.Static.Db.GetQuestionText(questionId)))
	} else {
		buffer.WriteString(data.Static.Trans("not_set"))
	}

	buffer.WriteString(data.Static.Trans("variants_caption"))
	if data.Static.Db.GetQuestionVariantsCount(questionId) > 0 {
		variants := data.Static.Db.GetQuestionVariants(questionId)

		for i, variant := range variants {
			buffer.WriteString(fmt.Sprintf("\n<i>%d</i> - %s", i+1, variant))
		}
	} else {
		buffer.WriteString(data.Static.Trans("not_set"))
	}

	buffer.WriteString(data.Static.Trans("rules_caption"))
	if data.Static.Db.IsQuestionHasRules(questionId) {
		minAnswers, maxAnswers, time := data.Static.Db.GetQuestionRules(questionId)
		buffer.WriteString(processing.GetQuestionRulesText(minAnswers, maxAnswers, time, "answers", data.Static.Trans))
	} else {
		buffer.WriteString(data.Static.Trans("not_set"))
	}
	
	return buffer.String()
}
