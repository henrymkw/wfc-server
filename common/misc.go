package common

func UNUSED(v ...interface{}) {
}

func ByteSliceToUint32(buf []byte) uint32 {
	return uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
}

func ByteSliceToUint64(buf []byte) uint64 {
	return uint64(buf[0])<<56 | uint64(buf[1])<<48 | uint64(buf[2])<<40 | uint64(buf[3])<<32 |
		uint64(buf[4])<<24 | uint64(buf[5])<<16 | uint64(buf[6])<<8 | uint64(buf[7])
}
