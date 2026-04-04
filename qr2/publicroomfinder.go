package qr2

import (
	"wwfc/common"
	"wwfc/logging"
)

func tryFindPublicRoomForPlayer(player *Player, region common.MKWServerSearchRegion, gameMode common.MKWServerGameMode) bool {
	for id, room := range rooms {
		if id == "" || room == nil {
			continue
		}

		if room.Region == common.Private {
			continue
		}

		// check that the rooms region is the same as the requested region
		if room.Region != region {
			continue
		}

		if room.gameMode != gameMode {
			continue
		}

		// continue if room is full, is suspended, or canceled
		if room.full() || room.suspended || room.canceled {
			continue
		}

		if room.mkwServer == nil {
			logging.Info(moduleName, "Room", room.roomID, "has a nil mkwServer for some reason (shouldn't happen)")
			continue
		}

		logging.Info(moduleName, "Found a public room Player", player.PlayerId, "can join!")

		// add the player to the room and return.
		// to support vr based searches, we would need to loop through all rooms before adding players
		addPlayerResult := room.tryAddPlayerToRoom(player, false)
		if !addPlayerResult {
			logging.Info(moduleName, "Room", id, "addPlayerToRoom() failed for player", player.PlayerId)
		}
		return addPlayerResult
	}

	// if we reach here, then there are no available rooms, so create a public room with the specified region
	logging.Info(moduleName, "No rooms specify search criteria, creating a room for Player", player.PlayerId)
	newRoom := createRoom(player, region, gameMode)
	if newRoom == nil {
		logging.Info(moduleName, "Player", player.PlayerId, "failed to create a public room of region", region)
	}
	return newRoom != nil

}
