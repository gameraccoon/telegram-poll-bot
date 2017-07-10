package dialogFactory

import (
	"github.com/gameraccoon/telegram-poll-bot/processing"
	"github.com/gameraccoon/telegram-poll-bot/dialogs"
)

type variantPrototype struct {
	name string
	// nil if the variant is always active
	isActiveFn func(data *processing.ProcessData) bool
	process func(data *processing.ProcessData)
}

type DialogFactory struct {
	getTextFn func(data *processing.ProcessData) string
	variants map[string]variantPrototype
}

func (dialogFactory *DialogFactory) MakeDialog(data *processing.ProcessData) string {
	dialog := Dialog {
		text : dialogFactory.getText(data),
		variants : dialogFactory.getVariants(data)
	}
}

func (dialogFactory *DialogFactory) getText(data *processing.ProcessData) string {
	if dialogFactory.getTextFn != nil {
		return dialogFactory.getTextFn(data)
	} else {
		return ""
	}
}

func (dialogFactory *DialogFactory) getVariants(data *processing.ProcessData) (variants map[string]string) {
	for id, variant := range dialogFactory.variants {
		if variant.IsActive() {
			variants[variant.name] = variant.text
		}
	}
}

func (dialog *DialogFactory) ProcessVariant(id string, data *processing.ProcessData) {
	variant, isVariantAvailable := dialog.variants[id]
	if isVariantAvailable {
		variant.process(data)
	}
}

func (variant *cariantPrototype) isActive(data *processing.ProcessData) bool {
	if variant.isActive != nil {
		return variant.isActive(data)
	}
	
	// return true because isActive hasn't set
	return true
}
