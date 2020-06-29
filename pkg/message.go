package rehook

type RegisterMessage struct {
	Keys []string `json:"keys"`
}

type RetransmitMessage struct {
	Key  string `json:"key"`
	ID   string `json:"id"`
	Body []byte `json:"body"`
}

type responseMessage struct {
	Error   error
	Message *RetransmitMessage
}
