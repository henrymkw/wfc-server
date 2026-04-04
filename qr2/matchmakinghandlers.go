package qr2

import (
	"wwfc/common"
	"wwfc/logging"

	"github.com/logrusorgru/aurora/v3"
)

// moduleName is used, want to still distinguish this in the logs

// request to open a private room
func handleOpenRoomRequest(host *Player) {
	// createFriendRoom() does room creation level validation
	room := createRoom(host, common.Private, common.None)
	if room == nil {
		logging.Error(moduleName, "Failed to create room for player with id", host.PlayerId, "at address", aurora.Cyan(host.Addr))
		return
	}
}

func handleJoinFroomRequest(guest *Player, request *JoinFroomRequest) {
	logging.Info(moduleName, "Player wants to join a private room with friend profile id", aurora.Cyan(request.friendProfileId))

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
		logging.Info(moduleName, "can't join room since host has no room!")
		return
	}

	if room.host != host {
		logging.Info(moduleName, "Rooms host isn't the expected host!")
		return
	}

	room.tryAddPlayerToRoom(guest, false)
}

func handleLeaveFroomRequest(player *Player) {
	room := player.roomPointer
	if room == nil {
		logging.Info(moduleName, "Player", player.Addr.String(), "sent a LeaveFroom request when they're roomless!")
		return
	}

	room.removePlayerFromRoom(player)
}

func handleSuspendRequest(player *Player, requestSuspend bool) {
	room := player.roomPointer
	if room == nil {
		logging.Info(moduleName, "Player", player.Addr.String(), "requested to suspend (value:", requestSuspend, ") but doesn't belong to a room")
		return
	}

	player.suspendVote = requestSuspend
}

func handleSearchPublicRoomRequest(player *Player, searchReq *SearchPublicRoomRequest) {
	if player.roomPointer != nil {
		logging.Info(moduleName, "Player", player.PlayerId, "requested to search for a room, but their roomPointer isn't nil")
		return
	}

	findRoomResult := tryFindPublicRoomForPlayer(player, searchReq.region, searchReq.mode)
	if !findRoomResult {
		logging.Info(moduleName, "tryFindPublicRoomForPlayer() failed for player", player.PlayerId)
		return
	}
}
