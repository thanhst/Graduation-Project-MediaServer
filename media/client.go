package media

import (
	"log"
	"mediaserver/media/message"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

type Client struct {
	UserID      string
	RoomID      string
	Role        string
	IsCamOn     bool
	IsMicOn     bool
	PeerConn    *webrtc.PeerConnection
	Conn        *websocket.Conn
	AudioTrack  *webrtc.TrackLocalStaticRTP
	VideoTrack  *webrtc.TrackLocalStaticRTP
	ScreenTrack *webrtc.TrackLocalStaticRTP
	Send        chan message.Message
	Read        chan message.Message
	Done        chan struct{}
	Streams     *[]interface{}
	CloseOnce   sync.Once
}

func CreateClientConnection(userId string, roomId string, role string, isCamOn bool, isMicOn bool, connection *websocket.Conn) *Client {
	log.Println("Create user")
	return &Client{
		UserID:  userId,
		RoomID:  roomId,
		Role:    role,
		Conn:    connection,
		IsCamOn: isCamOn,
		IsMicOn: isMicOn,
		Send:    make(chan message.Message, 256),
		Read:    make(chan message.Message, 256),
		Done:    make(chan struct{}),
	}
}

func ReadPump(user *Client, room *Room) {
	defer func() {
		if r := recover(); r != nil {
		}
		delete(room.Clients, user.UserID)
		log.Println("Delete user")
		user.CloseOnce.Do(func() {
			close(user.Done)
			close(user.Read)
			close(user.Send)
			user.Conn.Close()
		})
	}()
	for {
		var msg message.Message
		if err := user.Conn.ReadJSON(&msg); err != nil {
			log.Println("Error read:", err)
			break
		}
		user.Read <- msg
	}
}

func WritePump(user *Client) {
	for msg := range user.Send {
		log.Println(user.UserID, " send: ", msg.Event)
		err := user.Conn.WriteJSON(map[string]interface{}{
			"event":   msg.Event,
			"userId":  msg.UserID,
			"roomId":  msg.RoomID,
			"payload": msg.Payload,
		})
		if err != nil {
			log.Println(err)
			user.CloseOnce.Do(func() {
				close(user.Done)
				close(user.Read)
				close(user.Send)
				user.Conn.Close()
			})
			break
		}
	}
}
func (c *Client) SafeSend(msg message.Message) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered from send panic:", r)
		}
	}()
	select {
	case <-c.Done:
		return
	case c.Send <- msg:
	}
}
