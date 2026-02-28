package qr2

import (
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
	moduleName := "RoomManager: " + address

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

	if len(*buffer)+len(data) > 0x1000 {
		logging.Error(moduleName, "Buffer overflow")
		common.CloseConnection(ServerName, index)
		buffer = nil
		return
	}

	matchRequestHeader := tryParseMatchRequestHeader(data)
	if matchRequestHeader == nil {
		logging.Info(moduleName, "failed to parse match request header from", address)
		return
	}

	player := validateBasics(matchRequestHeader)
	if player == nil {
		logging.Info(moduleName, "Player failed basic validation for match request from", address)
		return
	}

	// TODO: Is this a good place to do this?
	player.roomManagerConnnectionIndex = index
	player.roomManagerAddr = address

	requestType := matchRequestHeader.requestType

	switch requestType {
	case OpenRoom:
		handleOpenRoomRequest(player)
	default:
		logging.Error(moduleName, "Unknown request type", aurora.Cyan(requestType))
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
