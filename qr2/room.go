package qr2

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/gob"
	"os"
	"strconv"

	"time"
	"wwfc/common"
	"wwfc/logging"

	"github.com/logrusorgru/aurora/v3"
)

type Room struct {
	roomID        uint32
	roomName      string
	CreateTime    time.Time
	Region        common.Region
	LastJoinIndex int
	host          *Player          // only applies for private rooms, authority of room settings
	players       map[*Player]bool // only added for non-guests

	RaceNumber    int
	CourseID      int
	EngineClassID int

	isFriendRoom         bool
	mkwServer            *MKWServer
	aidBitmap            uint32     // available aid bitmap
	numAids              uint32     // num non-guest players
	directAidBitmap      uint32     // aid bitmap, including guests.
	suspended            bool       // match making suspension state
	canceled             bool       // whether room is canceled
	aidLocalPlayerCounts [12]uint32 // local player counts for each player. Size is always 12, even if there isn't 12 players in the room
}

func createFriendRoom(host *Player) *Room {
	if !canPlayerCreateFriendRoom(host) {
		return nil
	}

	var localPlayerCounts [12]uint32
	localPlayerCounts[0] = common.SetLocalPlayerCount(host.localPlayerCount)

	id := generateRoomID()
	name := strconv.FormatUint(uint64(id), 16)

	room := &Room{
		roomID:               	id,
		roomName:             	name,
		CreateTime:           	time.Now(),
		Region:               	common.None,
		LastJoinIndex:        	0,
		host:                 	host,
		players:              	map[*Player]bool{host: true},
		RaceNumber:           	0,
		CourseID:             	-1,
		EngineClassID:     	   	-1,
		isFriendRoom:     	   	true,
		mkwServer:       	    nil,
		aidBitmap: 			  	0,
		numAids:          	    0,
		directAidBitmap: 		0,
		aidLocalPlayerCounts: localPlayerCounts,
	}

	mkwServer := createMKWServer(room)
	if mkwServer == nil {
		logging.Info(moduleName, "mkwServer creation failed")
		return nil
	}

	host.addPlayerToRoom(room)

	room.mkwServer = mkwServer

	// room.mkwServer.addPlayer(host)

	go func() {
		ticker := time.NewTicker(1000 * time.Millisecond)

		for {
			<-ticker.C

			mutex.Lock()
			room.broadcastMatchPackets()

			mutex.Unlock()
		}
	}()

	rooms[name] = room

	return room
}

// at this point, we've varified that friendsAddedOrOpenHost() returned true, the host is the rooms host according to both the room and player types
func (r *Room) joinFriendRoom(guest *Player) bool {
	// some verification before a player can join a room:

	if r.suspended {
		logging.Info(name, "Match making is suspended, cannot join currently!")
		return false
	}

	if r.canceled {
		logging.Info(name, "Room is canceled!")
		return false
	}

	joinResult := guest.addPlayerToRoom(r)

	return joinResult
}

func (r *Room) broadcastMatchPackets() {
	for p, exists := range r.players {
		if p == nil {
			continue
		}

		if !exists {
			continue
		}

		if p.roomManagerAddr == "" {
			continue
		}

		err := sendToAid(r.aidBitmap, r.numAids, r.directAidBitmap, r.roomID, r.host.aid, r.suspended, r.canceled, &r.aidLocalPlayerCounts, p.aid, p.roomManagerConnnectionIndex)
		if err != nil {
			p.numConsecutiveRoomManagerSendErrors += 1
		} else {
			p.numConsecutiveRoomManagerSendErrors = 0
		}
		if p.numConsecutiveRoomManagerSendErrors >= 5 {
			// gotta remove them from the room
			logging.Info(moduleName, "Player timed out, removing from room", aurora.Cyan(p.Addr.String()), "Aid", aurora.Cyan(p.aid))
			p.numConsecutiveRoomManagerSendErrors = 0
			p.removeFromRoom()
		}

	}
}

func (r *Room) removePlayer(p *Player) {
	// check the player is in the room
	if !r.players[p] {
		return
	}

	r.numAids -= p.localPlayerCount

	leaversAid := p.aid
	r.aidBitmap = common.ClearAid(r.aidBitmap, leaversAid)
	r.directAidBitmap = common.ClearAid(r.directAidBitmap, leaversAid)
	r.aidLocalPlayerCounts[leaversAid] = 0

	// delete the room in this case
	if p == r.host {
		r.host = nil
	}
	delete(r.players, p)
}

func (r *Room) isEmpty() bool {
	return r.numAids == 0
}

func (r *Room) isFull() bool {
	numPlayers := r.numPlayers()
	if numPlayers > 12 {
		// error case, this is bad
		logging.Error(moduleName, "BAD! Room", r.roomID, "has too many players! Num:", numPlayers)
	}
	return numPlayers == 12
}

// we can't go by numAids since an aid can have at most 2 players
func (r *Room) numPlayers() uint32 {
	var total uint32 = 0
	for p, exists := range r.players {
		if p != nil && exists {
			total += p.localPlayerCount
		}
	}
	return total
}

func ProcessGPStatusUpdate(profileID uint32, senderIP uint64, status string) {
	moduleName := "QR2/GPStatus:" + strconv.FormatUint(uint64(profileID), 10)

	mutex.Lock()
	defer mutex.Unlock()

	login, exists := logins[profileID]
	if !exists || login == nil {
		logging.Info(moduleName, "Received status update for non-existent profile ID", aurora.Cyan(profileID))
		return
	}

	player := login.player
	if player == nil {
		if senderIP == 0 {
			logging.Info(moduleName, "Received status update for profile ID", aurora.Cyan(profileID), "but no player exists")
			return
		}

		// Login with this profile ID
		player, exists = players[senderIP]
		if !exists || player == nil {
			logging.Info(moduleName, "Received status update for profile ID", aurora.Cyan(profileID), "but no player exists")
			return
		}

		if !player.setProfileID(moduleName, strconv.FormatUint(uint64(profileID), 10), "") {
			return
		}
	}

	// Send the client message exploit if not received yet
	if status != "0" && status != "1" && !player.ExploitReceived && player.login != nil && player.login.NeedsExploit {
		playerCopy := *player

		mutex.Unlock()
		logging.Notice(moduleName, "Sending SBCM exploit to DNS patcher client")
		sendClientExploit(moduleName, playerCopy)
		mutex.Lock()
	}

	if status == "0" || status == "1" || status == "3" || status == "4" {
		player := players[senderIP]
		if player == nil || player.roomPointer == nil {
			return
		}

		player.removeFromRoom()
	}
}

func ProcessUSER(senderPid uint32, senderIP uint64, packet []byte) {
	moduleName := "QR2:ProcessUSER/" + strconv.FormatUint(uint64(senderPid), 10)

	mutex.Lock()
	login := logins[senderPid]
	if login == nil {
		mutex.Unlock()
		logging.Warn(moduleName, "Received USER packet from non-existent profile ID", aurora.Cyan(senderPid))
		return
	}

	player := login.player
	if player == nil {
		mutex.Unlock()
		logging.Warn(moduleName, "Received USER packet from profile ID", aurora.Cyan(senderPid), "but no player exists")
		return
	}
	mutex.Unlock()

	miiroomCount := binary.BigEndian.Uint16(packet[0x04:0x06])
	if miiroomCount != 2 {
		logging.Error(moduleName, "Received USER packet with unexpected Mii room count", aurora.Cyan(miiroomCount))
		// Kick the client
		gpErrorCallback(senderPid, "bad_packet")
		return
	}

	miiroomBitflags := binary.BigEndian.Uint32(packet[0x00:0x04])

	var miiData []string
	var miiName []string
	for i := 0; i < int(miiroomCount); i++ {
		if miiroomBitflags&(1<<uint(i)) == 0 {
			continue
		}

		index := 0x08 + i*0x4C
		mii := common.Mii(packet[index : index+0x4C])
		if mii.RFLCalculateCRC() != 0x0000 {
			logging.Error(moduleName, "Received USER packet with invalid Mii data CRC")
			gpErrorCallback(senderPid, "bad_packet")
			return
		}

		createId := binary.BigEndian.Uint64(packet[index+0x18 : index+0x20])
		official, _ := common.RFLSearchOfficialData(createId)
		if official {
			miiName = append(miiName, "Player")
		} else {
			decodedName, err := common.GetWideString(packet[index+0x2:index+0x2+20], binary.BigEndian)
			if err != nil {
				logging.Error(moduleName, "Failed to parse Mii name:", err)
				gpErrorCallback(senderPid, "bad_packet")
				return
			}

			miiName = append(miiName, decodedName)
		}

		miiData = append(miiData, base64.StdEncoding.EncodeToString(packet[index:index+0x4A]))
	}

	mutex.Lock()
	defer mutex.Unlock()

	for i, name := range miiName {
		player.Data["+mii"+strconv.Itoa(i)] = miiData[i]
		player.Data["+mii_name"+strconv.Itoa(i)] = name
	}
}

func ProcessMKWSelectRecord(profileId uint32, key string, value string) {
	moduleName := "QR2:MKWSelectRecord:" + strconv.FormatUint(uint64(profileId), 10)

	mutex.Lock()
	login := logins[profileId]
	if login == nil {
		mutex.Unlock()
		logging.Warn(moduleName, "Received SELECT record from non-existent profile ID", aurora.Cyan(profileId))
		return
	}

	player := login.player
	if player == nil {
		mutex.Unlock()
		logging.Warn(moduleName, "Received SELECT record  from profile ID", aurora.Cyan(profileId), "but no player exists")
		return
	}
	mutex.Unlock()

	room := player.roomPointer
	if room == nil {
		return
	}

	keyColored := aurora.BrightCyan(key).String()

	switch key {
	case "wl:mkw_select_course":
		courseId, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			logging.Error(moduleName, "Error decoding", keyColored+":", err.Error())
			return
		}

		logging.Info(moduleName, "Selected course", aurora.BrightCyan(strconv.FormatUint(courseId, 10)))

		mutex.Lock()
		defer mutex.Unlock()

		room.RaceNumber++
		room.CourseID = int(courseId)
		room.EngineClassID = -1
		return

	case "wl:mkw_select_cc":
		ccId, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			logging.Error(moduleName, "Error decoding", keyColored+":", err.Error())
			return
		}

		logging.Info(moduleName, "Selected CC", aurora.BrightCyan(strconv.FormatUint(ccId, 10)))

		mutex.Lock()
		defer mutex.Unlock()

		room.EngineClassID = int(ccId)
		return
	}

}

// saveRooms saves the current rooms state to disk.
// Expects the mutex to be locked.
func saveRooms() error {
	file, err := os.OpenFile("state/qr2_rooms.gob", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	encoder := gob.NewEncoder(file)
	err = encoder.Encode(rooms)
	file.Close()
	return err
}

// loadRooms loads the rooms state from disk.
// Expects the mutex to be locked, and the players to already be loaded.
func loadRooms() error {
	file, err := os.Open("state/qr2_rooms.gob")
	if err != nil {
		return err
	}

	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&rooms)
	file.Close()
	if err != nil {
		return err
	}

	for _, player := range players {
		if player.roomPointer != nil || player.RoomName == "" {
			continue
		}

		room := rooms[player.RoomName]
		if room == nil {
			logging.Warn("QR2", "player", aurora.BrightCyan(player.Addr.String()), "has a room name but the room does not exist")
			continue
		}

		if room.players == nil {
			room.players = map[*Player]bool{}
		}

		room.players[player] = true
		player.roomPointer = room
	}

	return nil
}

func shutdownMKWServerServers() {
	for _, g := range rooms {
		if g.mkwServer != nil {
			if g.mkwServer.Cmd != nil && g.mkwServer.Cmd.Process != nil {
				logging.Info("QR2", "Shutting down mkw-server process for room", aurora.Cyan(g.roomName))
				g.mkwServer.Cmd.Process.Kill()
			}
		}
	}
}
