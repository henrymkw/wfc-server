package qr2

import (
	"errors"
	"fmt"

	"wwfc/common"
	"wwfc/logging"

	"github.com/logrusorgru/aurora/v3"
)

// request to open a private room
func handleOpenRoomRequest(host *Player) error {
	err := createRoom(host, common.Private, common.None)
	if err != nil {
		return fmt.Errorf("createRoom() failed for player %d with message %s", host.PlayerId, err.Error())
	}
	return nil
}

func handleJoinFriendRequest(joiner *Player, request *JoinFriendRequest) error {
	logging.Info(moduleName, "Player wants to join a private room with friend profile id", aurora.Cyan(request.friendProfileId), "and searchRegion", request.searchRegion)

	friendProfileId := request.friendProfileId

	canJoinRoom := joiner.friendsAddedOrOpenHost(friendProfileId)
	if !canJoinRoom {
		return fmt.Errorf("Player %d can't join friends room since they're not friends!", joiner.PlayerId)
	}
	// shouldn't be nil if friendsAddedOrOpenHost() returned true
	friend := logins[friendProfileId].player

	room := friend.roomPointer
	if room == nil {
		return errors.New("can't join room since host has no room!")
	}

	if room.Region != request.searchRegion {
		return fmt.Errorf("Can't join friend room due to mismatched search regions! Room's is region %d while joiner's is %d", room.Region, request.searchRegion)
	}

	if room.Region == common.Private && room.host != friend {
		return errors.New("Rooms host isn't the expected host!")
	}

	return room.tryAddPlayerToRoom(joiner, false)
}

func handleLeaveRoomRequest(player *Player) error {
	room := player.roomPointer
	if room == nil {
		return fmt.Errorf("Player %d sent a LeaveRoom request when they're roomless!")
	}

	return room.removePlayerFromRoom(player)
}

func handleSuspendRequest(player *Player, requestSuspend bool) error {
	room := player.roomPointer
	if room == nil {
		return fmt.Errorf("Player %d requested to suspend (value: %d ) but doesn't belong to a room", player.PlayerId, requestSuspend)
	}

	player.suspendVote = requestSuspend
	return nil
}

func handleSearchPublicRoomRequest(player *Player, searchReq *SearchPublicRoomRequest) error {
	if player.roomPointer != nil {
		return fmt.Errorf("Player %d requested to search for a room, but their roomPointer isn't nil", player.PlayerId)
	}

	return tryFindPublicRoomForPlayer(player, searchReq.region, searchReq.mode)
}

func handleSetLocalPlayerCount(player *Player, lpc uint8) error {
	err := player.setLocalPlayers(lpc)

	return err
}
