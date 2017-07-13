package processing

import (
	"github.com/gameraccoon/telegram-poll-bot/chat"
	"github.com/gameraccoon/telegram-poll-bot/database"
	"github.com/nicksnyder/go-i18n/i18n"
	"time"
)

type UserState int

const (
	Normal UserState = iota
	WaitingText
	WaitingVariants
	WaitingRules
)

type StaticConfiguration struct {
	Language    string
	Moderators  []int64
	ExtendedLog bool
}

type StaticProccessStructs struct {
	Chat       chat.Chat
	Db         *database.Database
	UserStates map[int64]UserState
	Timers     map[int64]time.Time
	Config     *StaticConfiguration
	Trans      i18n.TranslateFunc
}
