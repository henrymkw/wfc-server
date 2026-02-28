package qr2

import (
	"wwfc/logging"

	"github.com/logrusorgru/aurora/v3"
)

// moduleName is used, want to still distinguish this in the logs
var name = "MatchMakingRequestHandler"

// request to open a private room
func handleOpenRoomRequest(player *Player) {
	// createFriendRoom() does room creation level validation
	room := createFriendRoom(player)
	if room == nil {
		logging.Error(name, "Failed to create room for player with id", player.PlayerId, "at address", aurora.Cyan(player.Addr))
		return
	}

	logging.Info(name, "Player can open a private room!")
	player.roomPointer = room
	player.RoomName = room.roomName
	player.Aid = 0
	player.IsHost = true

}
