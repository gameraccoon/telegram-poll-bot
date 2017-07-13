package dialogFactories

import (
	"github.com/gameraccoon/telegram-poll-bot/dialog"
	"github.com/gameraccoon/telegram-poll-bot/processing"
)

type variantPrototype struct {
	text string
	// nil if the variant is always active
	isActiveFn func(data *processing.ProcessData) bool
	process    func(data *processing.ProcessData)
}

type DialogFactory struct {
	id        string
	getTextFn func(data *processing.ProcessData) string
	variants  map[string]variantPrototype
}

func (dialogFactory *DialogFactory) MakeDialog(data *processing.ProcessData) *dialog.Dialog {
	dialog := dialog.Dialog{
		Id:       dialogFactory.id,
		Text:     dialogFactory.getText(data),
		Variants: dialogFactory.getVariants(data),
	}
	return &dialog
}

func (dialog *DialogFactory) ProcessVariant(id string, data *processing.ProcessData) {
	variant, isVariantAvailable := dialog.variants[id]
	if isVariantAvailable {
		variant.process(data)
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
		if variant.isActive(data) {
			if variants == nil {
				variants = make(map[string]string)
			}
			variants[id] = variant.text
		}
	}
	return
}

func (variant *variantPrototype) isActive(data *processing.ProcessData) bool {
	if variant.isActiveFn != nil {
		return variant.isActiveFn(data)
	}

	// return true because isActiveFn hasn't set
	return true
}
