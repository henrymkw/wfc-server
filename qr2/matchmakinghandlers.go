package qr2

import (
	"wwfc/logging"

	"github.com/logrusorgru/aurora/v3"
)

// moduleName is used, want to still distinguish this in the logs
var name = "MatchMakingRequestHandler"

// request to open a private room
func handleOpenRoomRequest(host *Player) {
	// createFriendRoom() does room creation level validation
	room := createFriendRoom(host)
	if room == nil {
		logging.Error(name, "Failed to create room for player with id", host.PlayerId, "at address", aurora.Cyan(host.Addr))
		return
	}
}

func handleJoinFroomRequest(guest *Player, request *JoinFroomRequest) {
	logging.Info(name, "Player wants to join a private room with friend profile id", aurora.Cyan(request.friendProfileId))

	hostProfileId := request.friendProfileId

	canJoinRoom := guest.friendsAddedOrOpenHost(hostProfileId)

	if !canJoinRoom {
		return
	}
	// some verification before a player can join a room:
	// - we can get the host's underlying Player
	// - their roomPointer is non-nil and the roomPointer's host points to themhost := logins[hostProfileId].player

	// shouldn't be nil if friendsAddedOrOpenHost() returned true
	host := logins[hostProfileId].player

	room := host.roomPointer
	if room == nil {
		logging.Info(name, "can't join room since host has no room!")
		return
	}

	if room.host != host {
		logging.Info(name, "Rooms host isn't the expected host!")
		return
	}

	room.joinFriendRoom(guest)
}
