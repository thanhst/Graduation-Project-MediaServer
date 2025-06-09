package signaling

import (
	"log"
	"mediaserver/media"
	"mediaserver/media/message"
	"net/http"
)

func HandlerConnection(w http.ResponseWriter, r *http.Request) {
	log.Println("Connect")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error to connect this connection!")
		return
	}
	var msg message.Message
	if err := conn.ReadJSON(&msg); err != nil {
		log.Println("Init read error:", err)
		conn.Close()
		return
	}

	client := media.CreateClientConnection(msg.UserID, msg.RoomID, msg.Payload["role"].(string), msg.Payload["isCamOn"].(bool), msg.Payload["isMicOn"].(bool), conn)
	media.RoomsMutex.Lock()
	room, exists := media.Rooms[msg.RoomID]
	if !exists {
		room = media.CreateRoom(msg.RoomID, nil)
		media.Rooms[msg.RoomID] = room
	}
	go room.Run()
	media.RoomsMutex.Unlock()
	handleClientJoin(client, room)
}
