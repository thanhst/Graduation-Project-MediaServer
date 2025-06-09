package message

type Message struct {
	Event   string                 `json:"event"`
	UserID  string                 `json:"userId"`
	RoomID  string                 `json:"roomId"`
	Payload map[string]interface{} `json:"payload"`
}
