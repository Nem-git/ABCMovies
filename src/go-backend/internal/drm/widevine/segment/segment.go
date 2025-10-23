package segment

import (
	"fmt"
	"log"
	"strings"

	"github.com/Eyevinn/mp4ff/mp4"
)

func Get(initByte []byte, segmentByte []byte, keys []string, wantInit bool) ([]byte, error) {

	initMP4, err := decodeByteSegment(initByte)
	if err != nil {
		return nil, err
	}

	segmentMP4, err := decodeByteSegment(segmentByte)
	if err != nil {
		return nil, err
	}

	init, err := getInit(initMP4, segmentMP4)
	if err != nil {
		return nil, err
	}

	// TODO: FIXME: Check if that makes sense
	// if !segmentMP4.IsFragmented() {
	// 	return nil, fmt.Errorf("file not fragmented")
	// }

	decryptInfo, err := mp4.DecryptInit(init)
	if err != nil {
		return nil, fmt.Errorf("couldn't clean init: %w", err)
	}

	// Return decrypted init
	if wantInit {
		return encodeByteInit(init)
	}

	segmentMP4.Init = init

	// Decrypt segment
	for _, k := range keys {

		split := strings.Split(k, ":")
		if len(split) < 2 {
			log.Println("couldn't split key:", k)
			continue
		}
		k = split[0]

		key, err := mp4.UnpackKey(k)
		if err != nil {
			log.Println("couldn't unpack key")
			continue
		}

		for _, segment := range segmentMP4.Segments {
			if err := mp4.DecryptSegment(segment, decryptInfo, key); err != nil {
				log.Println("couldn't decrypt segment")
				continue
			}
		}
	}

	return encodeByteSegment(segmentMP4)
}
