package drm

import (
	"os"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func ivMustDecode(hexStr string) []byte {
	b := make([]byte, 0, len(hexStr)/2)
	for i := 0; i < len(hexStr); i += 2 {
		var v byte
		hi := hexVal(hexStr[i])
		lo := hexVal(hexStr[i+1])
		v = hi<<4 | lo
		b = append(b, v)
	}
	return b
}

func hexVal(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

func kidFromUUID(u mp4.UUID) KID {
	var kid KID
	copy(kid[:], u)
	return kid
}

var _ = testing.Short
