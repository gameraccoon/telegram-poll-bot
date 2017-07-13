package dialogFactories

import (
	"github.com/gameraccoon/telegram-poll-bot/dialog"
	"github.com/gameraccoon/telegram-poll-bot/processing"
)

type DialogManager struct {
	dialogs map[string]*DialogFactory
}

func (dialogManager *DialogManager) RegisterDialogFactory(id string, dialogFactory *DialogFactory) {
	if dialogManager.dialogs == nil {
		dialogManager.dialogs = make(map[string]*DialogFactory)
	}

	dialogManager.dialogs[id] = dialogFactory
	dialogFactory.id = id
}

func (dialogManager *DialogManager) MakeDialog(dialogId string, data *processing.ProcessData) (dialog *dialog.Dialog) {
	factory := dialogManager.getDialogFactory(dialogId)
	if factory != nil {
		dialog = factory.MakeDialog(data)
	}
	return
}

func (dialogManager *DialogManager) ProcessVariant(dialogId string, variantId string, data *processing.ProcessData) (processed bool) {
	factory := dialogManager.getDialogFactory(dialogId)
	if factory != nil {
		factory.ProcessVariant(variantId, data)
		processed = true
	}
	return
}

func (dialogManager *DialogManager) getDialogFactory(id string) *DialogFactory {
	dialogFactory, ok := dialogManager.dialogs[id]
	if ok && dialogFactory != nil {
		return dialogFactory
	} else {
		return nil
	}
}
