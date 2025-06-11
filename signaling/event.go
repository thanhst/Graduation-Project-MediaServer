package signaling

import (
	"fmt"
	"log"
	"mediaserver/media"
	"mediaserver/media/message"
	"sync"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

func handleClientJoin(client *media.Client, room *media.Room) {
	log.Println("Handle join")
	room.Mu.Lock()
	defer func() {
		room.Mu.Unlock()
		if r := recover(); r != nil {
		}
		log.Println("Unlock")
	}()
	if room.Clients[client.UserID] != nil {
		delete(room.Clients, client.UserID)
	}
	room.Clients[client.UserID] = client
	room.MsgChan <- &message.Message{
		Event:  "user-join",
		UserID: client.UserID,
		Payload: map[string]interface{}{
			"camState": client.IsCamOn,
			"micState": client.IsMicOn,
		},
	}
	go handleSignaling(client, room)
}

func handleSignaling(client *media.Client, room *media.Room) {
	log.Println("Handle signaling")
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered from send panic:", r)
		}
		room.Mu.Lock()
		handleDisconnect(client, room)
		room.Mu.Unlock()
	}()
	for msg := range client.Read {
		log.Println(client.UserID, " read : ", msg.Event)
		switch msg.Event {
		case "offer":
			offer, ok := msg.Payload["offer"].(map[string]interface{})
			if !ok {
				log.Println("Error to convert offer")
				return
			}
			sdpStr, _ := offer["sdp"].(string)
			if client.PeerConn == nil {
				err := CreatePeerConnection(client, room, &msg.Payload)
				if err != nil {
					log.Println("CreatePeerConnection error:", err)
					continue
				}
			} else {
				data := msg.Payload
				if data != nil {
					streams, ok := data["streams"].([]interface{})
					if ok {
						*client.Streams = streams
					}
				}

				offer := webrtc.SessionDescription{
					Type: webrtc.SDPTypeOffer,
					SDP:  sdpStr,
				}

				err := client.PeerConn.SetRemoteDescription(offer)
				if err != nil {
					log.Println("SetRemoteDescription error:", err)
					continue
				}

				answer, err := client.PeerConn.CreateAnswer(nil)
				if err != nil {
					log.Println("CreateAnswer error:", err)
					continue
				}

				err = client.PeerConn.SetLocalDescription(answer)
				if err != nil {
					log.Println("SetLocalDescription error:", err)
					continue
				}

				client.SafeSend(message.Message{
					Event: "answer",
					Payload: map[string]interface{}{
						"sdp": client.PeerConn.LocalDescription(),
					},
				})
			}

		case "ice-candidate":
			if client.PeerConn == nil {
				continue
			}

			candMap, ok := msg.Payload["candidate"].(map[string]interface{})
			if !ok {
				return
			}
			spdMid, _ := candMap["sdpMid"].(string)
			sdpMLineIndex := uint16(candMap["sdpMLineIndex"].(float64))

			candidate := webrtc.ICECandidateInit{
				Candidate:     candMap["candidate"].(string),
				SDPMid:        &spdMid,
				SDPMLineIndex: &sdpMLineIndex,
			}
			err := client.PeerConn.AddICECandidate(candidate)
			if err != nil {
				log.Println("AddICECandidate error:", err)
			}
		case "answer":
			answerData, ok := msg.Payload["sdp"].(string)
			if !ok {
				log.Println("Invalid answer payload")
				return
			}

			answer := webrtc.SessionDescription{
				Type: webrtc.SDPTypeAnswer,
				SDP:  answerData,
			}

			if client.PeerConn == nil {
				log.Println("PeerConn is nil when handling answer")
				return
			}

			err := client.PeerConn.SetRemoteDescription(answer)
			if err != nil {
				log.Println("SetRemoteDescription (answer) error:", err)
				return
			}

		case "switch-camera-micro":
			room.Mu.Lock()
			client := room.Clients[msg.UserID]
			room.Mu.Unlock()
			client.IsCamOn = msg.Payload["camState"].(bool)
			client.IsMicOn = msg.Payload["micState"].(bool)
			room.MsgChan <- &message.Message{
				Event:  "switch-camera-micro",
				UserID: msg.UserID,
				Payload: map[string]interface{}{
					"camState": msg.Payload["camState"].(bool),
					"micState": msg.Payload["micState"].(bool),
				},
			}

		case "request-pli":
			room.Mu.Lock()
			userSender := room.Clients[msg.UserID]
			room.Mu.Unlock()
			// handleReGetClients(client, room)
			sendPLIWhenReady(userSender.PeerConn)
		case "start-share":
			room.MsgChan <- &message.Message{
				Event:   "start-share",
				UserID:  msg.UserID,
				Payload: map[string]interface{}{},
			}
		case "stop-share":
			room.MsgChan <- &message.Message{
				Event:   "stop-share",
				UserID:  msg.UserID,
				Payload: map[string]interface{}{},
			}
		}
	}
}

func handleDisconnect(client *media.Client, room *media.Room) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered from send panic:", r)
		}
	}()
	log.Println("Disconnect")
	if client.PeerConn != nil {
		for _, sender := range client.PeerConn.GetSenders() {
			_ = client.PeerConn.RemoveTrack(sender)
		}
		client.PeerConn.Close()
	}
	room.MsgChan <- &message.Message{
		Event:   "user-leave",
		UserID:  client.UserID,
		RoomID:  room.ID,
		Payload: map[string]interface{}{},
	}
	client.Conn.Close()
	delete(room.Clients, client.UserID)
}

func CreatePeerConnection(client *media.Client, room *media.Room, payload *map[string]interface{}) error {
	// log.Println("Create new connection")
	offer, ok := (*payload)["offer"].(map[string]interface{})
	if !ok {
		log.Println("Error to convert offer")
		return nil
	}
	offerSDP, _ := offer["sdp"].(string)

	streams, ok := (*payload)["streams"].([]interface{})
	if ok {
		client.Streams = &streams
	}

	mediaEngine := webrtc.MediaEngine{}
	mediaEngine.RegisterDefaultCodecs()

	api := webrtc.NewAPI(webrtc.WithMediaEngine(&mediaEngine))
	pc, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			}, {
				URLs:       []string{"turn:openrelay.metered.ca:80"},
				Username:   "openrelayproject",
				Credential: "openrelayproject",
			},
		},
	})
	if err != nil {
		log.Println("Error to create peerConnection")
		return err
	}

	err = pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offerSDP,
	})
	if err != nil {
		return err
	}

	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			client.SafeSend(message.Message{
				Event: "ice-candidate",
				Payload: map[string]interface{}{
					"candidate": c.ToJSON(),
				},
			})
		}
	})
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return err
	}

	err = pc.SetLocalDescription(answer)
	if err != nil {
		return err
	}
	client.PeerConn = pc
	client.RoomID = room.ID
	client.SafeSend(message.Message{
		Event: "answer",
		Payload: map[string]interface{}{
			"sdp": *pc.LocalDescription(),
		},
	})

	pc.OnTrack(func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		fmt.Printf("SFU: Received %s track from %s\n", remoteTrack.Kind().String(), client.UserID)
		var typeTrack string
		for _, stream := range *client.Streams {
			streamMap, ok := stream.(map[string]interface{})
			if !ok {
				continue
			}
			if streamMap["trackId"].(string) == remoteTrack.ID() {
				typeVal, ok := streamMap["type"].(string)
				if ok {
					typeTrack = typeVal
				}
				break
			}
		}
		// Tạo local track tương ứng cùng kind (audio/video)
		localTrack, err := webrtc.NewTrackLocalStaticRTP(
			remoteTrack.Codec().RTPCodecCapability,
			remoteTrack.ID(),
			remoteTrack.StreamID())
		if err != nil {
			log.Println("SFU: failed to create local track", err)
			return
		}

		// Lưu track local trong client để giữ tham chiếu lâu dài (quan trọng!)
		if typeTrack == "audio" {
			client.AudioTrack = localTrack
		} else if typeTrack == "video" {
			client.VideoTrack = localTrack
		} else if typeTrack == "screen" {
			client.ScreenTrack = localTrack
		}
		// Đọc RTP từ remoteTrack, gửi đến localTrack (forward )
		go func() {
			rtpBuf := make([]byte, 4096)
			for {
				n, _, readErr := remoteTrack.Read(rtpBuf)
				if readErr != nil {
					log.Println("remoteTrack.Read error:", readErr)
					break
				}
				// log.Println("SFU: read RTP packet with length", n)

				_, writeErr := localTrack.Write(rtpBuf[:n])
				if writeErr != nil {
					log.Println("localTrack.Write error:", writeErr)
					break
				}

				// log.Printf("SFU: forwarded %d bytes to localTrack (written: %d)\n", n, written)
			}
		}()

		// **FIX: Broadcast track to all existing clients and trigger renegotiation**
		var clientsToRenegotiate []*media.Client
		func() {
			room.Mu.Lock()
			defer room.Mu.Unlock()
			for _, other := range room.Clients {
				if other.UserID == client.UserID || other.PeerConn == nil {
					continue
				}

				var addedTrack *webrtc.TrackLocalStaticRTP
				if typeTrack == "audio" {
					addedTrack = client.AudioTrack
				} else if typeTrack == "video" {
					addedTrack = client.VideoTrack
				} else if typeTrack == "screen" {
					addedTrack = client.ScreenTrack
				}

				if addedTrack != nil {
					_, err := other.PeerConn.AddTrack(addedTrack)
					if err != nil {
						log.Printf("SFU: failed to add %s track to %s: %v\n", remoteTrack.Kind().String(), other.UserID, err)
						continue
					}
					// Collect clients that need renegotiation
					clientsToRenegotiate = append(clientsToRenegotiate, other)
				}
			}
		}()

		// **FIX: Renegotiate with all existing clients to make them see the new track**
		for _, otherClient := range clientsToRenegotiate {
			go renegotiate(otherClient)
		}

		room.MsgChan <- &message.Message{
			Event:  "new-stream",
			UserID: client.UserID,
			RoomID: room.ID,
			Payload: map[string]interface{}{
				"type":     typeTrack,
				"trackId":  localTrack.ID(),
				"streamId": localTrack.StreamID(),
			},
		}
		room.MsgChan <- &message.Message{
			Event:  "switch-camera-micro",
			UserID: client.UserID,
			RoomID: room.ID,
			Payload: map[string]interface{}{
				"camState": client.IsCamOn,
				"micState": client.IsMicOn,
			},
		}

		// **FIX: Send PLI only when connection is ready**
		// go func() {
		// 	time.Sleep(500 * time.Millisecond) // Wait for renegotiation to complete
		// 	sendPLIWhenReady(client.PeerConn)
		// }()
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateConnected {
			log.Printf("Client %s peer connection connected", client.UserID)
			handleGetTrackFromClients(client, room)
			var userStates []map[string]interface{}

			for _, other := range room.Clients {
				if client.UserID != other.UserID {
					userStates = append(userStates, map[string]interface{}{
						"userId":   other.UserID,
						"camState": other.IsCamOn,
						"micState": other.IsMicOn,
					})
				}
			}
			if len(userStates) > 0 {
				client.Send <- message.Message{
					Event: "get-all-user-states",
					Payload: map[string]interface{}{
						"users": userStates,
					},
				}
			}
			// **FIX: Send PLI after a small delay to ensure everything is ready**
			go func() {
				time.Sleep(1 * time.Second)
				func() {
					room.Mu.Lock()
					defer room.Mu.Unlock()
					for _, other := range room.Clients {
						if other.PeerConn != nil {
							sendPLIWhenReady(other.PeerConn)
						}
					}
				}()
			}()
		}
	})

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("Client %s ICE connection state: %s", client.UserID, state.String())
		if state == webrtc.ICEConnectionStateConnected {
			log.Printf("ICE connected for client %s, scheduling PLI", client.UserID)
			// **FIX: Send PLI after ICE is connected and stable**
			go func() {
				time.Sleep(1 * time.Second)
				room.Mu.Lock()
				defer room.Mu.Unlock()
				for _, other := range room.Clients {
					if other.PeerConn != nil {
						sendPLIWhenReady(other.PeerConn)
					}
				}
			}()
		}
	})

	// **FIX: Reduced frequency and added connection state check**
	go func() {
		ticker := time.NewTicker(3 * time.Second) // Reduced frequency
		defer ticker.Stop()

		for range ticker.C {
			func() {
				room.Mu.Lock()
				defer room.Mu.Unlock()
				if client.PeerConn != nil && client.PeerConn.ConnectionState() == webrtc.PeerConnectionStateConnected {
					sendPLIWhenReady(client.PeerConn)
				}

				for _, other := range room.Clients {
					if other.PeerConn != nil && other.PeerConn.ConnectionState() == webrtc.PeerConnectionStateConnected {
						sendPLIWhenReady(other.PeerConn)
					}
				}
			}()
		}
	}()

	return nil
}

func handleGetTrackFromClients(client *media.Client, room *media.Room) {
	log.Printf("Getting existing tracks for client %s", client.UserID)
	var hasTracksToAdd bool

	for _, other := range room.Clients {
		if other.UserID == client.UserID || other.PeerConn == nil {
			continue
		}

		if other.AudioTrack != nil {
			client.SafeSend(message.Message{
				Event:  "new-stream",
				UserID: other.UserID,
				RoomID: room.ID,
				Payload: map[string]interface{}{
					"type":     "audio",
					"trackId":  other.AudioTrack.ID(),
					"streamId": other.AudioTrack.StreamID(),
				},
			})
			_, err := client.PeerConn.AddTrack(other.AudioTrack)
			if err != nil {
				log.Println("Failed to add existing audio track:", err)
			} else {
				hasTracksToAdd = true
			}
		}
		if other.VideoTrack != nil {
			client.SafeSend(message.Message{
				Event:  "new-stream",
				UserID: other.UserID,
				RoomID: room.ID,
				Payload: map[string]interface{}{
					"type":     "video",
					"trackId":  other.VideoTrack.ID(),
					"streamId": other.VideoTrack.StreamID(),
				},
			})
			_, err := client.PeerConn.AddTrack(other.VideoTrack)
			if err != nil {
				log.Println("Failed to add existing video track:", err)
			} else {
				hasTracksToAdd = true
			}
		}
		if other.ScreenTrack != nil {
			client.SafeSend(message.Message{
				Event:  "new-stream",
				UserID: other.UserID,
				RoomID: room.ID,
				Payload: map[string]interface{}{
					"type":     "screen",
					"trackId":  other.ScreenTrack.ID(),
					"streamId": other.ScreenTrack.StreamID(),
				},
			})
			_, err := client.PeerConn.AddTrack(other.ScreenTrack)
			if err != nil {
				log.Println("Failed to add existing screen track:", err)
			} else {
				hasTracksToAdd = true
			}
		}
	}

	// Only renegotiate if we actually added tracks
	if hasTracksToAdd {
		renegotiate(client)
	}

	// **FIX: Send PLI after everything is set up**
	go func() {
		time.Sleep(1 * time.Second)
		func() {
			room.Mu.Lock()
			defer room.Mu.Unlock()
			for _, other := range room.Clients {
				if other.PeerConn != nil {
					sendPLIWhenReady(other.PeerConn)
				}
			}
		}()
	}()
}

// **FIX: New function to send PLI only when connection is ready**
func sendPLIWhenReady(pc *webrtc.PeerConnection) {
	if pc == nil {
		return
	}

	// Check if peer connection is in a good state
	if pc.ConnectionState() != webrtc.PeerConnectionStateConnected {
		log.Printf("Skipping PLI - connection state: %s", pc.ConnectionState().String())
		return
	}

	if pc.ICEConnectionState() != webrtc.ICEConnectionStateConnected &&
		pc.ICEConnectionState() != webrtc.ICEConnectionStateCompleted {
		log.Printf("Skipping PLI - ICE state: %s", pc.ICEConnectionState().String())
		return
	}

	sendPLI(pc)
}

// send lại frame ban đầu của stream cho user khác thay vì phải chờ frame mới.
func sendPLI(pc *webrtc.PeerConnection) {
	receivers := pc.GetReceivers()
	if len(receivers) == 0 {
		return
	}

	var wg sync.WaitGroup
	for _, receiver := range receivers {
		track := receiver.Track()
		if track == nil {
			continue
		}

		wg.Add(1)
		go func(t webrtc.TrackRemote) {
			defer wg.Done()
			err := pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(t.SSRC())}})
			if err != nil {
				log.Printf("sendPLI error for track %s: %v", t.ID(), err)
			}
		}(*track)
	}
	wg.Wait()
}

// tái đàm phán
func renegotiate(client *media.Client) {
	if client.PeerConn == nil {
		log.Printf("Cannot renegotiate - no peer connection for client %s", client.UserID)
		return
	}
	for {
		if client.PeerConn.SignalingState() == webrtc.SignalingStateStable {
			break
		}
	}

	if client.PeerConn.ConnectionState() != webrtc.PeerConnectionStateConnected {
		log.Printf("Skipping renegotiation - connection not ready for client %s", client.UserID)
		return
	}

	log.Printf("Starting renegotiation for client %s", client.UserID)

	offer, err := client.PeerConn.CreateOffer(nil)
	if err != nil {
		log.Printf("CreateOffer failed for client %s: %v", client.UserID, err)
		return
	}
	err = client.PeerConn.SetLocalDescription(offer)
	if err != nil {
		log.Printf("SetLocalDescription failed for client %s: %v", client.UserID, err)
		return
	}

	client.SafeSend(message.Message{
		Event: "offer",
		Payload: map[string]interface{}{
			"sdp":  offer.SDP,
			"type": offer.Type.String(),
		},
	})

	log.Printf("Renegotiation offer sent to client %s", client.UserID)
}
func generateTrackID(userID, trackType string) string {
	return fmt.Sprintf("%s_%s", userID, trackType)
}
