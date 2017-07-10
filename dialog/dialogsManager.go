package dialog

import (
	"github.com/gameraccoon/telegram-poll-bot/processing"
)

type DialogManager struct {
	dialogs map[string]*Dialog
}

func (dialogManager *DialogManager) RegisterDialog(id string, dialog *Dialog) {
	dialogsManager.dialogs[id] = dialog
}

func (dialogManager *DialogManager) GetDialog(sting id) *Dialog {
	dialog, ok := dialogManager.dialogs[id]
	if ok && dialog != nil {
		return &dialog
	} else {
		return nil
	}
}
