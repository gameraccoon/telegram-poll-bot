package dialogFactories

type DialogManager struct {
	dialogs map[string]*DialogFactory
}

func (dialogManager *DialogManager) RegisterDialogFactory(id string, dialogFactory *DialogFactory) {
	dialogManager.dialogs[id] = dialogFactory
}

func (dialogManager *DialogManager) GetDialogFactory(id string) *DialogFactory {
	dialogFactory, ok := dialogManager.dialogs[id]
	if ok && dialogFactory != nil {
		return dialogFactory
	} else {
		return nil
	}
}
