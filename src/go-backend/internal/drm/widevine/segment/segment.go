package segment

import (
	"encoding/base64"
	"fmt"
	"log"
	"strings"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func Get(initStr string, segmentStr string, keys []string, isInit bool) (string, error) {

	if segmentStr == "" {
		return "", fmt.Errorf("no segment segment provided")
	}

	segByte, err := base64.StdEncoding.DecodeString(segmentStr)
	if err != nil {
		return "", fmt.Errorf("couldn't base64 decode segment: %w", err)
	}

	log.Println(isInit, len(segByte))

	sr := bits.NewFixedSliceReader(segByte)

	segMp4, err := mp4.DecodeFileSR(sr)
	if err != nil {
		return "", fmt.Errorf("couldn't decode segment: %w", err)
	}

	if !segMp4.IsFragmented() {
		return "", fmt.Errorf("file not fragmented")
	}

	var init *mp4.InitSegment

	if segMp4.Init != nil {
		init = segMp4.Init
	} else {

		if initStr == "" {
			log.Println(segmentStr, isInit) // TODO: FIXME: Remove
			return "", fmt.Errorf("no init segment provided")
		}

		initByte, err := base64.StdEncoding.DecodeString(initStr)
		if err != nil {
			return "", fmt.Errorf("couldn't base64 decode init: %w", err)
		}

		sr := bits.NewFixedSliceReader(initByte)

		initMp4, err := mp4.DecodeFileSR(sr)
		if err != nil {
			return "", fmt.Errorf("couldn't decode init: %w", err)
		}

		if initMp4.Init == nil {
			//log.Println(initMp4.Init, keys) // TODO: FIXME: Remove
			return "", fmt.Errorf("init provided does not contain necessary atoms")
		}

		init = initMp4.Init
	}

	decryptInfo, err := mp4.DecryptInit(init)
	if err != nil {
		return "", fmt.Errorf("couldn't clean init: %w", err)
	}

	if isInit || segMp4.Init != nil {
		// if err := init.Encode; err != nil {
		// 	return "", fmt.Errorf("couldn't encode cleaned segment init to mp4")
		// }

		if isInit {
			sw := bits.NewFixedSliceWriter(int(init.Size()))
			init.EncodeSW(sw)
			encodedInit := base64.StdEncoding.EncodeToString(sw.Bytes())
			return encodedInit, nil
		}
	}

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

		for _, segment := range segMp4.Segments {
			if err := mp4.DecryptSegment(segment, decryptInfo, key); err != nil {
				log.Println("couldn't decrypt segment")
				continue
			}
		}
	}

	segMp4.Init = init

	sw := bits.NewFixedSliceWriter(int(segMp4.Size()))

	for _, segment := range segMp4.Segments {
		if err := segment.EncodeSW(sw); err != nil {
			return "", fmt.Errorf("couldn't encode segment")
		}
	}

	log.Println(isInit, segMp4.Size())

	// init.EncodeSW(sw)
	// segMp4.EncodeSW(sw)
	encodedInit := base64.StdEncoding.EncodeToString(sw.Bytes())
	return encodedInit, nil
}
