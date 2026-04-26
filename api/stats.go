package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"wwfc/common"
	"wwfc/qr2"
)

type Stats struct {
	OnlinePlayerCount int `json:"online"`
	ActivePlayerCount int `json:"active"`
	RoomCount        int `json:"rooms"`
}

func HandleStats(w http.ResponseWriter, r *http.Request) {
	u, err := url.Parse(r.URL.String())
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	games := query["game"]

	stats := map[string]Stats{}

	servers := qr2.GetPlayerServers()
	rooms := qr2.GetRooms([]string{}, []string{}, false)

	globalStats := Stats{
		OnlinePlayerCount: len(servers),
		ActivePlayerCount: 0,
		RoomCount:        len(rooms),
	}

	for _, server := range servers {
		gameName := server["gamename"]

		if server["+joinindex"] != "" {
			globalStats.ActivePlayerCount += 1
		}

		if len(games) > 0 && !common.StringInSlice(gameName, games) {
			continue
		}

		gameStats, exists := stats[gameName]
		if !exists {
			gameStats = Stats{
				OnlinePlayerCount: 0,
				ActivePlayerCount: 0,
				RoomCount:        0,
			}

			gameStats.RoomCount += len(rooms)
		}

		gameStats.OnlinePlayerCount += 1
		if server["+joinindex"] != "" {
			gameStats.ActivePlayerCount += 1
		}

		stats[gameName] = gameStats
	}

	stats["global"] = globalStats

	jsonData, err := json.Marshal(stats)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Length", strconv.Itoa(len(jsonData)))
	w.Write(jsonData)
}
