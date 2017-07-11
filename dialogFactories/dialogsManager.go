package dialogFactories

import (
	//"github.com/gameraccoon/telegram-poll-bot/processing"
	"github.com/gameraccoon/telegram-poll-bot/dialog"
)

type DialogManager struct {
	dialogs map[string]*dialog.Dialog
}

func (dialogManager *DialogManager) RegisterDialog(id string, dialog *Dialog) {
	dialogManager.dialogs[id] = dialog
}

func (dialogManager *DialogManager) GetDialog(id string) *dialog.Dialog {
	dialog, ok := dialogManager.dialogs[id]
	if ok && dialog != nil {
		return dialog
	} else {
		return nil
	}
}
