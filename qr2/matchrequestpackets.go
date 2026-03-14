package qr2

import (
	"wwfc/common"
	"wwfc/logging"

	"github.com/logrusorgru/aurora/v3"
)

type MatchRequestHeader struct {
	Magic       uint32 // "MREQ"
	requestType MatchRequestType
	padding     [3]uint8
	searchId    uint64
}

type JoinFroomRequest struct {
	header          MatchRequestHeader
	friendProfileId uint32
}

type SuspendRequest struct {
	header         MatchRequestHeader
	suspendRequest bool
}

type MatchRequestType uint8

const (
	OpenFroom  = 0
	JoinFroom  = 1
	LeaveFroom = 2
	Suspend    = 3
)

func tryParseMatchRequestHeader(data []byte) *MatchRequestHeader {
	if len(data) != 16 {
		logging.Info(name, "Received packet with invalid length", aurora.Cyan(len(data)), "expected 16")
		return nil
	}

	magic := data[:4]
	if string(magic) != "MREQ" {
		logging.Info(name, "Received packet with invalid magic", aurora.Cyan(string(magic)), "expected MREQ")
		return nil
	}

	matchRequest := uint8(data[4])

	return &MatchRequestHeader{
		Magic:       0x77826981, // no need to be fancy about converting, we know its valid so just hardcode
		requestType: MatchRequestType(matchRequest),
		searchId:    common.ByteSliceToUint64(data[8:16]),
	}
}
