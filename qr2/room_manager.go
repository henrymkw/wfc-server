package qr2

import (
	"encoding/binary"
	"math/rand"

	"wwfc/common"
	"wwfc/logging"

	"github.com/logrusorgru/aurora/v3"
	"github.com/sasha-s/go-deadlock"
)

var ServerName = "roommanager"
var rooms = map[string]*Room{}

var (
	connBuffers = map[uint64]*[]byte{}
	mutexRM     = deadlock.RWMutex{}
)

func NewConnection(index uint64, address string) {
	mutexRM.Lock()
	connBuffers[index] = &[]byte{}
	mutexRM.Unlock()
}

func CloseConnection(index uint64) {
	mutexRM.Lock()
	delete(connBuffers, index)
	mutexRM.Unlock()
}

func HandlePacket(index uint64, data []byte, address string) {
	mutexRM.RLock()
	buffer := connBuffers[index]
	mutexRM.RUnlock()

	if buffer == nil {
		buffer = &[]byte{}
		defer func() {
			if buffer == nil {
				return
			}

			mutexRM.Lock()
			connBuffers[index] = buffer
			mutexRM.Unlock()
		}()
	}

	*buffer = append(*buffer, data...)

	if len(*buffer)+len(data) > 0x1000 {
		logging.Error(moduleName, "Buffer overflow")
		common.CloseConnection(ServerName, index)
		buffer = nil
		return
	}

	// 0x10 being the minimum size for a complete match packet (header size)
	for len(*buffer) >= 0x10 {
		matchRequestHeader := tryParseMatchRequestHeader((*buffer)[:0x10])
		if matchRequestHeader == nil {
			logging.Info(moduleName, "failed to parse match request header from", address)
			return
		}

		player := validateBasics(matchRequestHeader)
		if player == nil {
			logging.Info(moduleName, "Player failed basic validation for match request from", address)
			// close connection if basic validation fails
			common.CloseConnection(ServerName, index)
			buffer = nil
			return
		}

		// store connection information needed to send messages back to the player
		player.setRoomManagerConnection(index, address)

		requestType := matchRequestHeader.requestType

		// get the expected packet length for each message type
		var msgLen int
		switch requestType {
		case OpenFroom, LeaveRoom:
			msgLen = 0x10
		case JoinFriend, Suspend, SearchPublicRoom, LocalPlayerCount:
			msgLen = 0x18
		default:
			logging.Info(moduleName, "Unknown request type sent by player", player.PlayerId, "type:", requestType)
			*buffer = (*buffer)[:0]
		}

		// data got cut off or message is invalid, break and wait until we recv again
		if len(*buffer) < msgLen {
			break
		}

		// we can process a complete message, update buffer and process the message
		msg := (*buffer)[:msgLen]
		*buffer = (*buffer)[msgLen:]

		switch requestType {
		case OpenFroom:
			logging.Info(moduleName, "Received OpenRoom request from", address)
			err := handleOpenRoomRequest(player)
			if err != nil {
				logging.Info(moduleName, err.Error())
			}

		case JoinFriend:
			logging.Info(moduleName, "Received JoinFroom request from", address)

			req := &JoinFriendRequest{
				header:          *matchRequestHeader,
				friendProfileId: binary.BigEndian.Uint32(msg[0x10:0x14]),
				searchRegion:    common.MKWServerSearchRegion(msg[0x14]),
			}

			err := handleJoinFriendRequest(player, req)
			if err != nil {
				logging.Info(moduleName, "JoinFriend failed. Reason:", err.Error())
			}
		case LeaveRoom:
			logging.Info(moduleName, "Received LeaveRoom request from", address)
			err := handleLeaveRoomRequest(player)
			if err != nil {
				logging.Info(moduleName, "LeaveRoom failed for reason:", err.Error())
			}

		case Suspend:
			suspendRequest := msg[0x10] != 0
			err := handleSuspendRequest(player, suspendRequest)
			if err != nil {
				logging.Info(moduleName, "Suspend failed for reason:", err.Error())
			}

		case SearchPublicRoom:
			searchReq := &SearchPublicRoomRequest{
				header: *matchRequestHeader,
				region: common.MKWServerSearchRegion(msg[0x10]),
				mode:   common.MKWServerGameMode(msg[0x11]),
			}
			logging.Info(moduleName, "Received SearchPublicRoom from", address)
			err := handleSearchPublicRoomRequest(player, searchReq)
			if err != nil {
				logging.Info(moduleName, "SearchPublicRoom failed with reason:", err.Error())
			}

		case LocalPlayerCount:
			// Simple packet structure, just get the localPlayerCount from offset 0x10
			localPlayerCount := msg[0x10]

			logging.Info(moduleName, "Received LocalPlayerCount from", address, "where localPlayers is", localPlayerCount)

			err := handleSetLocalPlayerCount(player, localPlayerCount)
			if err != nil {
				logging.Info(moduleName, err.Error())
			}

		default:
			logging.Error(moduleName, "Unknown request type", aurora.Cyan(requestType))
		}
	}
}

// validates basic requirements to even make a match making request
// more can be added here
func validateBasics(header *MatchRequestHeader) *Player {
	// convert to int so we can check if the player exists
	player, _ := playerBySearchID[header.searchId]
	if player == nil {
		logging.Info(moduleName, "No player with searchId", header.searchId, "exists")
		return nil
	}

	// validate as much as we can, check for Authenticated, ExploitReceived, roomPointer == nil, etc
	if !player.Authenticated {
		logging.Info(moduleName, "PlayerId", player.PlayerId, "is not authenticated")
		return nil
	}

	return player
}

func generateRoomID() uint32 {
	return rand.Uint32()
}
