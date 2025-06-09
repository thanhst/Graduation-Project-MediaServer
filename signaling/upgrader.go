package signaling

import (
	"mediaserver/utils/dotenv"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  65536,
	WriteBufferSize: 65536,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == dotenv.GetDotEnv("FE_URL")+":"+dotenv.GetDotEnv("FE_PORT")
	},
}
