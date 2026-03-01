package qr2

import (
	"encoding/gob"
	"os"
	"strconv"

	"wwfc/common"
)

type LoginInfo struct {
	ProfileID           uint32
	GameCode            string
	InGameName          string
	ConsoleFriendCode   uint64
	FriendKeyGame       string
	GPPublicIP          string
	NeedsExploit        bool
	DeviceAuthenticated bool
	Restricted          bool
	player              *Player

	friendsList [30]uint32
	OpenHost    bool
}

var logins = map[uint32]*LoginInfo{}

func Login(profileID uint32, gameCode string, inGameName string, consoleFriendCode uint64, fcGame string, publicIP string, needsExploit bool, deviceAuthenticated bool, restricted bool, openHost bool) {
	mutex.Lock()
	defer mutex.Unlock()

	logins[profileID] = &LoginInfo{
		ProfileID:           profileID,
		GameCode:            gameCode,
		InGameName:          inGameName,
		ConsoleFriendCode:   consoleFriendCode,
		FriendKeyGame:       fcGame,
		GPPublicIP:          publicIP,
		NeedsExploit:        needsExploit,
		DeviceAuthenticated: deviceAuthenticated,
		Restricted:          restricted,
		player:              nil,

		OpenHost: openHost,
	}
}

func SetDeviceAuthenticated(profileID uint32) {
	mutex.Lock()
	defer mutex.Unlock()

	if login, exists := logins[profileID]; exists {
		login.DeviceAuthenticated = true
		if login.player != nil {
			login.player.Data["+deviceauth"] = "1"
		}
	}
}

func Logout(profileID uint32) {
	mutex.Lock()
	defer mutex.Unlock()

	// Delete login's player
	if login, exists := logins[profileID]; exists {
		if login.player != nil {
			removePlayer(common.MakeLookupAddr(login.player.Addr.String()))
		}
	}

	delete(logins, profileID)
}

// Save logins to a file. Expects the mutex to be locked.
func saveLogins() error {
	file, err := os.OpenFile("state/qr2_logins.gob", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	encoder := gob.NewEncoder(file)
	err = encoder.Encode(logins)
	file.Close()
	return err
}

// Load logins from a file. Expects the mutex to be locked, and the players to already be loaded.
func loadLogins() error {
	file, err := os.Open("state/qr2_logins.gob")
	if err != nil {
		return err
	}

	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&logins)
	file.Close()
	if err != nil {
		return err
	}

	for _, player := range players {
		dwcPid := player.Data["dwc_pid"]
		if dwcPid == "" {
			continue
		}

		profileID, err := strconv.ParseUint(dwcPid, 10, 32)
		if err != nil {
			continue
		}

		if login, exists := logins[uint32(profileID)]; exists {
			login.player = player
			player.login = login
		}
	}

	return nil
}

func AddToFriendsList(profileId uint32, friendId uint32) {
	mutex.Lock()
	defer mutex.Unlock()

	if login, exists := logins[profileId]; exists {
		for i := range login.friendsList {
			if login.friendsList[i] == 0 {
				login.friendsList[i] = friendId
				break
			}
		}
	}
}

func RemoveFromFriendList(profileId uint32, friendId uint32) {
	mutex.Lock()
	defer mutex.Unlock()

	if login, exists := logins[profileId]; exists {
		for i := range login.friendsList {
			if login.friendsList[i] == friendId {
				login.friendsList[i] = 0
				break
			}
		}
	}
}
