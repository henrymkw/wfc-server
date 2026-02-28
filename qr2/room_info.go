package qr2

import (
	"sort"
	"strconv"
	"time"
	"wwfc/common"
)

type MiiInfo struct {
	MiiData string `json:"data"`
	MiiName string `json:"name"`
}

type PlayerInfo struct {
	Count      string `json:"count"`
	ProfileID  string `json:"pid"`
	InGameName string `json:"name"`
	ConnMap    string `json:"conn_map"`
	ConnFail   string `json:"conn_fail"`
	Suspend    string `json:"suspend"`

	// Mario Kart Wii-specific fields
	FriendCode string    `json:"fc,omitempty"`
	VersusELO  string    `json:"ev,omitempty"`
	BattleELO  string    `json:"eb,omitempty"`
	Mii        []MiiInfo `json:"mii,omitempty"`
}

type RoomInfo struct {
	RoomName    string        `json:"id"`
	CreateTime  time.Time     `json:"created"`
	MatchType   string        `json:"type"`
	Suspend     bool          `json:"suspend"`
	ServerIndex string        `json:"host,omitempty"`
	MKWRegion   common.Region `json:"rk,omitempty"`

	Players  map[string]PlayerInfo `json:"players"`
	RaceInfo *RaceInfo             `json:"race,omitempty"`

	PlayersRaw      map[string]map[string]string `json:"-"`
	SortedJoinIndex []string                     `json:"-"`
}

type RaceInfo struct {
	RaceNumber    int `json:"num"`
	CourseID      int `json:"course"`
	EngineClassID int `json:"cc"`
}

func getRoomsRaw(gameNames []string, roomNames []string) []RoomInfo {
	var roomsCopy []RoomInfo

	mutex.Lock()
	defer mutex.Unlock()

	for _, room := range rooms {
		roomInfo := RoomInfo{
			RoomName:        room.roomName,
			CreateTime:      room.CreateTime,
			MatchType:       "",
			Suspend:         true,
			ServerIndex:     "",
			MKWRegion:       common.None,
			Players:         map[string]PlayerInfo{},
			PlayersRaw:      map[string]map[string]string{},
			SortedJoinIndex: []string{},
		}

		if room.isFriendRoom {
			roomInfo.MatchType = "private"
		} else {
			roomInfo.MatchType = "anybody"
		}

		roomInfo.MKWRegion = room.Region

		if room.RaceNumber != 0 {
			roomInfo.RaceInfo = &RaceInfo{
				RaceNumber:    room.RaceNumber,
				CourseID:      room.CourseID,
				EngineClassID: room.EngineClassID,
			}
		}

		for player := range room.players {
			mapData := map[string]string{}
			for k, v := range player.Data {
				mapData[k] = v
			}

			if login := player.login; login != nil {
				mapData["+ingamesn"] = login.InGameName
			} else {
				mapData["+ingamesn"] = ""
			}

			roomInfo.PlayersRaw[mapData["+joinindex"]] = mapData

			if mapData["dwc_hoststate"] == "2" && mapData["dwc_suspend"] == "0" {
				roomInfo.Suspend = false
			}

			// Add the join index to the sorted list
			myJoinIndex, _ := strconv.Atoi(mapData["+joinindex"])
			added := false

			for i, joinIndex := range roomInfo.SortedJoinIndex {
				if joinIndex == mapData["+joinindex"] {
					added = true
					break
				}

				intJoinIndex, _ := strconv.Atoi(joinIndex)
				if intJoinIndex > myJoinIndex {
					roomInfo.SortedJoinIndex = append(roomInfo.SortedJoinIndex, "")
					copy(roomInfo.SortedJoinIndex[i+1:], roomInfo.SortedJoinIndex[i:])
					roomInfo.SortedJoinIndex[i] = mapData["+joinindex"]
					added = true
					break
				}
			}

			if !added {
				roomInfo.SortedJoinIndex = append(roomInfo.SortedJoinIndex, mapData["+joinindex"])
			}
		}

		roomsCopy = append(roomsCopy, roomInfo)
	}

	return roomsCopy
}

// GetRooms returns a copy of all online rooms
func GetRooms(gameNames []string, roomNames []string, sorted bool) []RoomInfo {
	roomsCopy := getRoomsRaw(gameNames, roomNames)

	for i, room := range roomsCopy {
		for joinIndex, rawPlayer := range room.PlayersRaw {
			playerInfo := PlayerInfo{
				Count:      rawPlayer["+localplayers"],
				ProfileID:  rawPlayer["dwc_pid"],
				InGameName: rawPlayer["+ingamesn"],
			}

			pid, err := strconv.ParseUint(rawPlayer["dwc_pid"], 10, 32)
			if err == nil {
				if fcGame := rawPlayer["+fcgameid"]; len(fcGame) == 4 {
					playerInfo.FriendCode = common.CalcFriendCodeString(uint32(pid), fcGame)
				}
			}

			if rawPlayer["gamename"] == "mariokartwii" {
				playerInfo.VersusELO = rawPlayer["ev"]
				playerInfo.BattleELO = rawPlayer["eb"]
			}

			for i := 0; i < 32; i++ {
				miiData := rawPlayer["+mii"+strconv.Itoa(i)]
				if miiData == "" {
					continue
				}

				playerInfo.Mii = append(playerInfo.Mii, MiiInfo{
					MiiData: miiData,
					MiiName: rawPlayer["+mii_name"+strconv.Itoa(i)],
				})
			}

			for _, newIndex := range room.SortedJoinIndex {
				if newIndex == joinIndex {
					continue
				}

				if rawPlayer["+conn_"+newIndex] == "" {
					playerInfo.ConnMap += "0"
					continue
				}

				playerInfo.ConnMap += rawPlayer["+conn_"+newIndex]
			}

			playerInfo.ConnFail = rawPlayer["+conn_fail"]
			if playerInfo.ConnFail == "" {
				playerInfo.ConnFail = "0"
			}

			playerInfo.Suspend = rawPlayer["dwc_suspend"]

			roomsCopy[i].Players[joinIndex] = playerInfo
		}
	}

	if sorted {
		sort.Slice(roomsCopy, func(i, j int) bool {
			if roomsCopy[i].CreateTime.Equal(roomsCopy[j].CreateTime) {
				return roomsCopy[i].RoomName < roomsCopy[j].RoomName
			}

			return roomsCopy[i].CreateTime.Before(roomsCopy[j].CreateTime)
		})
	}

	return roomsCopy
}
