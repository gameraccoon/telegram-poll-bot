package processing

type ProcessData struct {
	Static  *StaticProccessStructs
	Command string // first part of command without slash(/)
	Message string // parameters of command or plain message
	ChatId  int64
	UserId  int64
}
