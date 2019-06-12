package leanstreams

import (
	"encoding/binary"
)

func byteArrayToInt64(bytes []byte) (result int64, bytesRead int) {
	return binary.Varint(bytes)
}

func int64ToByteArray(value int64, bufferSize int) []byte {
	toWriteLen := make([]byte, bufferSize)
	binary.PutVarint(toWriteLen, value)
	return toWriteLen
}
