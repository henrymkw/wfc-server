package qr2

import (
	"fmt"

	"wwfc/common"
	"wwfc/logging"
)

func findPublicRoom(player *Player, region common.MKWServerSearchRegion, gameMode common.MKWServerGameMode) error {
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

		logging.Info(moduleName, "Found a public room Player", player.PlayerId, "can join!")

		// to support vr based searches, we would need to loop through all rooms before adding players
		err := room.tryAddPlayer(player, false)
		if err != nil {
			logging.Info(moduleName, "findPublicRoom(): %s", err.Error(), "Continuing room search")
			continue

		}
	}

	// if we reach here, then there are no available rooms, so create a public room with the specified region
	logging.Info(moduleName, "No rooms specify search criteria, creating a room for Player", player.PlayerId)
	err := createRoom(player, region, gameMode)
	if err != nil {
		return fmt.Errorf("Player %d failed to create a public room. createRoom() returned %s", player.PlayerId, err.Error())
	}
	return nil

}
