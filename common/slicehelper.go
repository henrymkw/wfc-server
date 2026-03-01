package common

import "errors"

func SliceOfFourToUint32(b []byte) (uint32, error) {
	if len(b) != 4 {
		return 0, errors.New("slice must be exactly 4 bytes long")
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), nil
}

func UnpackPort(b []byte) (int, error) {
	if len(b) != 2 {
		return 0, errors.New("slice must be exactly 2 bytes long")
	}
	return int(b[0]) << 8 | int(b[1]), nil
}
