package qr2

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
	"wwfc/common"
)

func sendChallenge(conn net.PacketConn, addr net.UDPAddr, player Player, lookupAddr uint64) {
	challenge := player.Challenge
	if challenge == "" {
		// Generate challenge
		addrString := strings.Split(addr.String(), ":")
		var hexIP string
		for _, i := range strings.Split(addrString[0], ".") {
			val, err := strconv.ParseUint(i, 10, 64)
			if err != nil {
				panic(err)
			}

			hexIP += fmt.Sprintf("%02X", val)
		}

		port, err := strconv.ParseUint(addrString[1], 10, 64)
		if err != nil {
			panic(err)
		}

		hexPort := fmt.Sprintf("%04X", port)

		challenge = common.RandomString(6) + "00" + hexIP + hexPort
		mutex.Lock()
		if playerPtr := players[lookupAddr]; playerPtr != nil {
			playerPtr.Challenge = challenge
		} else {
			mutex.Unlock()
			return
		}
		mutex.Unlock()
	}

	response := createResponseHeader(ChallengeRequest, player.PlayerID)
	response = append(response, []byte(challenge)...)
	response = append(response, 0)

	go func() {
		for {
			conn.WriteTo(response, &addr)

			time.Sleep(1 * time.Second)

			mutex.Lock()
			player, ok := players[lookupAddr]
			if !ok || player.Authenticated || player.LastKeepAlive < time.Now().UTC().Unix()-60 {
				mutex.Unlock()
				return
			}
			addr = player.Addr
			mutex.Unlock()
		}
	}()
}
