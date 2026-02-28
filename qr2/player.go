package qr2

import (
	"encoding/gob"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
	"wwfc/common"
	"wwfc/logging"

	"github.com/logrusorgru/aurora/v3"
	"github.com/sasha-s/go-deadlock"
	"gvisor.dev/gvisor/pkg/sleep"
)

const (
	ClientLittleEndian = 0
	ClientBigEndian    = 1
	ClientNoEndian     = 2
)

type Player struct {
	PlayerId        uint32
	SearchId        uint64
	Addr            net.UDPAddr
	Challenge       string
	Authenticated   bool
	login           *LoginInfo
	ExploitReceived bool
	LastKeepAlive   int64
	Data            map[string]string
	PacketCount     uint32
	messageMutex    *deadlock.Mutex
	messageAckWaker *sleep.Waker
	roomPointer     *Room
	RoomName        string

	recvSearchId    bool
	searchIdGuesses uint32 // attempt to prevent brute forcing the searchId

	Aid         uint8 // only set when in a room
	IsHost      bool  // only set when in a room
	HasGuest    bool  // set when player has a guest
	SuspendVote bool  // vote to suspend match making.
	is2Players  bool

	roomManagerConnnectionIndex         uint64
	roomManagerAddr                     string // address roommanager sends to
	numConsecutiveRoomManagerSendErrors uint32
}

var (
	players          = map[uint64]*Player{}
	playerBySearchID = map[uint64]*Player{}
	mutex            = deadlock.Mutex{}
)

func addPlayerToRoom(p *Player, r *Room) bool {
	// check if the room empty (just started)
	if r.isEmpty() {
		r.players[p] = true
		p.roomPointer = r
		p.RoomName = r.roomName
		p.Aid = 0
		p.IsHost = true
		p.SuspendVote = false
		return true
	} else if r.isFull() {
		return false
	} else {
		// TODO: Add guest to friend room
		return false
	}

}

// Remove a player. Expects the global mutex to already be locked.
func removePlayer(addr uint64) {
	player := players[addr]
	if player == nil {
		return
	}

	player.messageAckWaker.Assert()

	if player.roomPointer != nil {
		player.removeFromRoom()
	}

	if player.login != nil {
		player.login.player = nil
		player.login = nil
	}

	// Delete search ID lookup
	delete(playerBySearchID, players[addr].SearchId)

	delete(players, addr)
}

// Remove player from room. Expects the global mutex to already be locked.
func (player *Player) removeFromRoom() {
	if player.roomPointer == nil {
		return
	}

	room := player.roomPointer
	delete(room.players, player)

	matchPacket := room.matchPacket
	if matchPacket != nil {
		matchPacket.removeAid(player.Aid)
	}

	if len(room.players) == 0 {
		logging.Notice("QR2", "Deleting room", aurora.Cyan(room.roomName))
		if room.mkwServerProxy != nil {
			logging.Notice("QR2", "Terminating mkw-server process for room", aurora.Cyan(room.roomName))
			if room.mkwServerProxy.Cmd != nil && room.mkwServerProxy.Cmd.Process != nil {
				logging.Notice("QR2", "Terminating mkw-server process with PID", aurora.Cyan(room.mkwServerProxy.Cmd.Process.Pid))
				room.mkwServerProxy.Cmd.Process.Signal(syscall.SIGTERM)
			} else {
				logging.Notice("QR2", "No mkw-server process found for room", aurora.Cyan(room.roomName))
			}

		}
		delete(rooms, room.roomName)
	}

	for player := range room.players {
		delete(player.Data, "+conn_"+player.Data["+joinindex"])
	}

	for field := range player.Data {
		if strings.HasPrefix(field, "+conn_") {
			delete(player.Data, field)
		}
	}

	mkwServerProxy := room.mkwServerProxy
	if mkwServerProxy != nil {
		mkwServerProxy.sendMkwServerRemoveClient(player)
	}

	player.roomPointer = nil
	player.RoomName = ""
}

// Update player data, creating the player if it doesn't exist. Returns a copy of the player data.
func setPlayerData(moduleName string, addr net.Addr, playerId uint32, payload map[string]string) (Player, bool) {
	newPID, newPIDValid := payload["dwc_pid"]
	delete(payload, "dwc_pid")

	lookupAddr := common.MakeLoopupAddr(addr.String())

	// Moving into performing operations on the player data, so lock the mutex
	mutex.Lock()
	defer mutex.Unlock()
	player, playerExists := players[lookupAddr]

	if playerExists && player.Addr.String() != addr.String() {
		logging.Error(moduleName, "player IP mismatch")
		return Player{}, false
	}

	if !playerExists {
		logging.Info(moduleName, "creating player in qr2 with addr", addr.String())
		player = &Player{
			PlayerId:        playerId,
			Addr:            *addr.(*net.UDPAddr),
			Challenge:       "",
			Authenticated:   false,
			LastKeepAlive:   time.Now().UTC().Unix(),
			Data:            payload,
			PacketCount:     0,
			messageMutex:    &deadlock.Mutex{},
			messageAckWaker: &sleep.Waker{},
			recvSearchId:    false,
			searchIdGuesses: 0,
		}
	}

	if newPIDValid && !player.setProfileID(moduleName, newPID, "") {
		return Player{}, false
	}

	if !playerExists {
		logging.Info(moduleName, "Creating playerId", aurora.Cyan(playerId).String(), "for", addr.String())

		// Set search ID
		for {
			searchID := uint64(rand.Int63n((1<<24)-1) + 1)
			if _, exists := playerBySearchID[searchID]; !exists {
				player.SearchId = searchID
				player.Data["+searchid"] = strconv.FormatUint(searchID, 10)
				playerBySearchID[searchID] = player
				break
			}
		}

		players[lookupAddr] = player
		return *player, true
	}

	// Save certain fields
	for k, v := range player.Data {
		if k[0] == '+' || k == "dwc_pid" {
			payload[k] = v
		}
	}

	player.Data = payload
	player.LastKeepAlive = time.Now().UTC().Unix()
	player.PlayerId = playerId
	return *player, true
}

// Set the player's profile ID if it doesn't already exists.
// Returns false if the profile ID is invalid.
// Expects the global mutex to already be locked.
func (player *Player) setProfileID(moduleName string, newPID string, gpcmIP string) bool {
	if oldPID, oldPIDValid := player.Data["dwc_pid"]; oldPIDValid && oldPID != "" {
		if newPID != oldPID {
			logging.Error(moduleName, "New dwc_pid mismatch: new:", aurora.Cyan(newPID), "old:", aurora.Cyan(oldPID))
			return false
		}

		return true
	}

	// Setting a new PID so validate it
	profileID, err := strconv.ParseUint(newPID, 10, 32)
	if err != nil || strconv.FormatUint(profileID, 10) != newPID {
		logging.Error(moduleName, "Invalid dwc_pid value:", aurora.Cyan(newPID))
		return false
	}

	// Check if the public IP matches the one used for the GPCM player
	var gpPublicIP string
	var loginInfo *LoginInfo
	var ok bool
	if loginInfo, ok = logins[uint32(profileID)]; ok {
		gpPublicIP = strings.Split(loginInfo.GPPublicIP, ":")[0]
	} else {
		logging.Error(moduleName, "Provided dwc_pid is not logged in:", aurora.Cyan(newPID))
		return false
	}

	// TODO: Some kind of authentication
	if gpcmIP != "" && gpcmIP != gpPublicIP {
		logging.Error(moduleName, "TCP public IP mismatch: SB:", aurora.Cyan(gpcmIP), "GP:", aurora.Cyan(gpPublicIP))
		return false
	}

	if ratingError := checkValidRating(moduleName, player.Data); ratingError != "ok" {
		profileId := loginInfo.ProfileID

		mutex.Unlock()
		gpErrorCallback(profileId, ratingError)
		mutex.Lock()
		return false
	}

	player.login = loginInfo

	// Constraint: only one player can exist with a given profile ID
	if loginInfo.player != nil {
		logging.Notice(moduleName, "Removing outdated player", aurora.BrightCyan(loginInfo.player.Addr.String()), "with PID", aurora.Cyan(newPID))
		removePlayer(common.MakeLoopupAddr(loginInfo.player.Addr.String()))
	}

	loginInfo.player = player

	if loginInfo.DeviceAuthenticated {
		player.Data["+deviceauth"] = "1"
	} else {
		player.Data["+deviceauth"] = "0"
	}

	player.Data["+gppublicip"], _ = common.IPFormatToString(gpPublicIP)
	player.Data["+fcgameid"] = loginInfo.FriendKeyGame

	player.Data["dwc_pid"] = newPID
	logging.Notice(moduleName, "Opened player with PID", aurora.Cyan(newPID))

	return true
}

func DoesPlayerExist(addr string) bool {
	logging.Info("QR2", "Checking player existence for", aurora.Cyan(addr))
	mutex.Lock()
	_, playerExists := players[common.MakeLoopupAddr(addr)]
	mutex.Unlock()
	return playerExists
}

func IsPlayerInRoom(addr string) bool {
	mutex.Lock()
	player, _ := players[common.MakeLoopupAddr(addr)]
	mutex.Unlock()

	if player == nil || player.roomPointer == nil {
		return false
	}

	return player.roomPointer != nil
}

// Get a copy of the list of servers
func GetPlayerServers() []map[string]string {
	var servers []map[string]string
	var unreachable []uint64
	currentTime := time.Now().UTC().Unix()

	mutex.Lock()
	defer mutex.Unlock()
	for playerAddr, player := range players {
		// If the last keep alive was over a minute ago then consider the server unreachable
		if player.LastKeepAlive < currentTime-60 {
			// If the last keep alive was over an hour ago then remove the server
			if player.LastKeepAlive < currentTime-((60*60)*1) {
				unreachable = append(unreachable, playerAddr)
			}
			continue
		}

		if !player.Authenticated {
			continue
		}

		servers = append(servers, player.Data)
	}

	// Remove unreachable players
	for _, playerAddr := range unreachable {
		logging.Notice("QR2", "Removing unreachable player", aurora.BrightCyan(players[playerAddr].Addr.String()))
		removePlayer(playerAddr)
	}

	return servers
}

func GetSearchID(addr uint64) uint64 {
	mutex.Lock()
	defer mutex.Unlock()

	if player := players[addr]; player != nil {
		return player.SearchId
	}

	return 0
}

// this assumes validateBasics() has been called
func canPlayerCreateFriendRoom(player *Player) bool {
	if player == nil {
		logging.Info("Room", "Player is nil, cannot create room")
		return false
	}

	if player.roomPointer != nil {
		logging.Info("Room", "Player is already in a room, cannot create room")
		return false
	}
	return true
}

// Save the players to a file. Expects the mutex to be locked.
func savePlayers() error {
	file, err := os.OpenFile("state/qr2_players.gob", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	encoder := gob.NewEncoder(file)
	err = encoder.Encode(players)
	file.Close()
	return err
}

// Load the players from a file. Expects the mutex to be locked.
func loadPlayers() error {
	file, err := os.Open("state/qr2_players.gob")
	if err != nil {
		return err
	}

	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&players)
	file.Close()
	if err != nil {
		return err
	}

	for _, players := range players {
		if players.SearchId != 0 {
			playerBySearchID[players.SearchId] = players
		}

		players.messageMutex = &deadlock.Mutex{}
		players.messageAckWaker = &sleep.Waker{}
		players.roomPointer = nil
		players.login = nil
	}

	return nil
}

func (player *Player) GetProfileId() string {
	return player.Data["dwc_pid"]
}
