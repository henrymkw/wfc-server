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

	ticker *time.Ticker
}

func createFriendRoom(host *Player) *Room {
	if host == nil {
		logging.Info(moduleName, "Host is nil! Can't create room!")
		return nil
	}

	if !canPlayerCreateFriendRoom(host) {
		return nil
	}

	id := generateRoomID()
	name := strconv.FormatUint(uint64(id), 16)

	room := &Room{
		roomID:               id,
		roomName:             name,
		CreateTime:           time.Now(),
		Region:               common.None,
		LastJoinIndex:        0,
		host:                 host,
		players:              map[*Player]bool{host: true},
		RaceNumber:           0,
		CourseID:             -1,
		EngineClassID:        -1,
		isFriendRoom:         true,
		mkwServer:            nil,
		aidBitmap:            0,
		numAids:              0,
		directAidBitmap:      0,
		aidLocalPlayerCounts: [12]uint32{},
	}

	mkwServer := startMKWServer(room)
	if mkwServer == nil {
		logging.Info(moduleName, "mkwServer creation failed")
		return nil
	}
	room.mkwServer = mkwServer

	room.addPlayerToRoom(host)

	// start broadcasting
	go func() {
		// not super crucial, but could look into the timer being configurable
		ticker := time.NewTicker(1000 * time.Millisecond)
		room.ticker = ticker

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

func (r *Room) addPlayerToRoom(p *Player) bool {
	if r.full() {
		logging.Info(moduleName, "Can't join room, room is full!")
		return false
	}

	// need to find the next available aid
	aid, err := common.GetAvailableAid(r.aidBitmap)
	if err != nil || aid == 0xff {
		logging.Info(moduleName, "GetAvailableAid() errored!")
	}

	// if the room is empty, this player is the host
	isHost := r.empty()

	r.players[p] = true
	r.aidBitmap = common.SetAid(r.aidBitmap, aid)
	r.directAidBitmap = common.SetAid(r.directAidBitmap, aid)
	r.numAids++
	r.aidLocalPlayerCounts[aid] = common.SetLocalPlayerCount(p.localPlayerCount)

	p.setRoomInfo(r, aid, isHost)

	// send a JoinFroom message only when the joining player is a guest.
	// This is because hosts have a different way of being established with
	// MKW-Server in handleMessageFromMKWServer
	if !isHost {
		r.mkwServer.sendJoinFroom(p)
	}
	return true
}

func (r *Room) removePlayerFromRoom(p *Player) {
	if p == nil {
		logging.Info(moduleName, "Can't remove a nil player!")
		return
	}

	if !r.players[p] {
		logging.Info(moduleName, "Can't remove player", p.PlayerId, "from room, doesn't exist")
		return
	}

	// check if the player leaving is the host, close room if so
	if p == r.host {
		logging.Info(moduleName, "Host left room. Attempting to close it!")
		r.close()
		return
	}

	// at this point, a guest is leaving, update the room accordingly
	leaversAid := p.aid

	r.numAids -= 1
	r.aidLocalPlayerCounts[leaversAid] = 0

	r.aidBitmap = common.ClearAid(r.aidBitmap, leaversAid)
	r.directAidBitmap = common.ClearAid(r.directAidBitmap, leaversAid)

	r.mkwServer.sendLeaveRoom(p)
	p.resetRoomInfo()

	delete(r.players, p)
}

// at this point, we've varified that friendsAddedOrOpenHost() returned true, the host is the rooms host according to both the room and player types
func (r *Room) joinFriendRoom(guest *Player) bool {
	if r.suspended {
		logging.Info(name, "Match making is suspended, cannot join currently!")
		return false
	}

	if r.canceled {
		logging.Info(name, "Room is canceled!")
		return false
	}

	joinResult := r.addPlayerToRoom(guest)

	return joinResult
}

func (r *Room) broadcastMatchPackets() {
	for p, exists := range r.players {
		if p == nil || !exists {
			continue
		}

		if p.roomManagerAddr == "" {
			continue
		}

		if r.host == nil {
			logging.Info(moduleName, "host is nil")
			continue
		}

		err := SendToAid(r.aidBitmap, r.numAids, r.directAidBitmap, r.roomID, r.host.aid, r.suspended, r.canceled, &r.aidLocalPlayerCounts, p.aid, p.roomManagerConnnectionIndex)
		if err != nil {
			p.numConsecutiveRoomManagerSendErrors += 1
		} else {
			p.numConsecutiveRoomManagerSendErrors = 0
		}
		if p.numConsecutiveRoomManagerSendErrors >= 5 {
			// gotta remove them from the room
			logging.Info(moduleName, "Player timed out, removing from room", aurora.Cyan(p.Addr.String()), "Aid", aurora.Cyan(p.aid))
			p.numConsecutiveRoomManagerSendErrors = 0
			logging.Info(moduleName, "removeFromRoom() called!!!!!!")
			r.removePlayerFromRoom(p)
		}

	}
}

func (r *Room) close() {
	// update the room to an 'empty state', then broadcast it.
	// this must be done before actually deleting it since the players need to be informed
	// after being broadcasted, the players will see the disconnected from room popup
	r.aidBitmap = 0
	r.numAids = 0
	r.directAidBitmap = 0
	r.roomID = 0
	r.suspended = false
	r.canceled = true
	r.aidLocalPlayerCounts = [12]uint32{}

	r.broadcastMatchPackets()

	// reset the room related info for the players in the room
	// this is necessary to allow them to join/create rooms again
	for p, exists := range r.players {
		if p == nil {
			continue
		}

		if !exists {
			continue
		}

		p.resetRoomInfo()
	}

	mkwServer := r.mkwServer
	if mkwServer == nil {
		logging.Info(moduleName, "Room's MKW-Server is nil, can't terminate")
		return
	}

	mkwServer.terminateProcess()

	r.ticker.Stop()

	name := r.roomName
	delete(rooms, name)
}

func (r *Room) empty() bool {
	return r.numAids == 0
}

func (r *Room) full() bool {
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
	for _, r := range rooms {
		mkwServer := r.mkwServer
		if mkwServer != nil {
			mkwServer.terminateProcess()
		}
	}
}
