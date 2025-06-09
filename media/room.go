package media

import (
	"mediaserver/media/message"
	"sync"

	"github.com/pion/webrtc/v3"
)

type Room struct {
	ID        string
	ShareConn *webrtc.PeerConnection
	Clients   map[string]*Client
	MsgChan   chan *message.Message
	Mu        sync.RWMutex
	QuitChan  chan struct{}
}

var Rooms = make(map[string]*Room)
var RoomsMutex sync.RWMutex

func CreateRoom(roomID string, shareConn *webrtc.PeerConnection) *Room {
	room := &Room{
		ID:        roomID,
		ShareConn: shareConn,
		MsgChan:   make(chan *message.Message),
		Clients:   make(map[string]*Client),
		QuitChan:  make(chan struct{}),
	}
	Rooms[roomID] = room
	return room
}

func (r *Room) Run() {
	for {
		select {
		case msg := <-r.MsgChan:
			r.broadcast(msg)
		case <-r.QuitChan:
			return
		}
	}
}
func (r *Room) broadcast(msg *message.Message) {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	for _, c := range r.Clients {
		if c.UserID == msg.UserID {
			continue
		}
		c.Send <- *msg
	}
}
