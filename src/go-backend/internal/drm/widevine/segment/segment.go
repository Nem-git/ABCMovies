package segment

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func Get(initStr string, segmentStr string, keys []string, wantInit bool) (string, error) {

	initMP4, err := decodeSegment(initStr)
	if err != nil {
		return "", err
	}

	segmentMP4, err := decodeSegment(segmentStr)
	if err != nil {
		return "", err
	}

	init, err := getInit(initMP4, segmentMP4)
	if err != nil {
		return "", err
	}

	if !segmentMP4.IsFragmented() {
		return "", fmt.Errorf("file not fragmented")
	}

	decryptInfo, err := mp4.DecryptInit(init)
	if err != nil {
		return "", fmt.Errorf("couldn't clean init: %w", err)
	}

	// Return decrypted init
	if wantInit {
		sw := bits.NewFixedSliceWriter(int(init.Size()))

		if err = segmentMP4.EncodeSW(sw); err != nil {
			return "", errors.New("couldn't encode mp4 segment")
		}

		return encodeInit(init)
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

	return encodeSegment(segmentMP4)
}

func getInit(initMP4 *mp4.File, segMP4 *mp4.File) (*mp4.InitSegment, error) {

	// Get init from segment
	if segMP4 != nil {
		if segMP4.Init != nil {
			return segMP4.Init, nil
		}
	}

	// Get init from init (lol)
	if initMP4 != nil {
		if initMP4.Init != nil {
			return initMP4.Init, nil
		}
	}

	return nil, fmt.Errorf("no init segment provided")
}

func decodeSegment(segment string) (*mp4.File, error) {

	segmentByte, err := base64.StdEncoding.DecodeString(segment)
	if err != nil {
		return nil, fmt.Errorf("couldn't base64 decode segment")
	}

	sr := bits.NewFixedSliceReader(segmentByte)

	segmentMP4, err := mp4.DecodeFileSR(sr)
	if err != nil {
		return nil, fmt.Errorf("couldn't decode init: %w", err)
	}

	return segmentMP4, nil
}

func encodeInit(init *mp4.InitSegment) (string, error) {

	sw := bits.NewFixedSliceWriter(int(init.Size()))

	if err := init.EncodeSW(sw); err != nil {
		return "", errors.New("couldn't encode mp4 init segment")
	}

	return base64.StdEncoding.EncodeToString(sw.Bytes()), nil
}

func encodeSegment(segmentMP4 *mp4.File) (string, error) {

	sw := bits.NewFixedSliceWriter(int(segmentMP4.Size()))

	if err := segmentMP4.EncodeSW(sw); err != nil {
		return "", errors.New("couldn't encode mp4 segment")
	}

	return base64.StdEncoding.EncodeToString(sw.Bytes()), nil
}
