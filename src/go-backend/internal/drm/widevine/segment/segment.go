package segment

import (
	"encoding/base64"
	"fmt"
	"log"
	"strings"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func Get(initStr string, segmentStr string, keys []string) (string, error) {

	if initStr == "" {
		return "", fmt.Errorf("no init segment provided")
	}

	initByte, err := base64.StdEncoding.DecodeString(initStr)
	if err != nil {
		return "", fmt.Errorf("couldn't decode init: %w", err)
	}

	sr := bits.NewFixedSliceReader(initByte)

	inputFile, err := mp4.DecodeFileSR(sr)
	if err != nil {
		return "", fmt.Errorf("couldn't parse init: %w", err)
	}

	init := inputFile.Init
	if inputFile.Init == nil {
		return "", fmt.Errorf("couldn't transform init to mp4.Init")
	}

	di, err := mp4.DecryptInit(init)
	if err != nil {
		return "", fmt.Errorf("couldn't clean init: %w", err)
	}

	if segmentStr == "" {
		sw := bits.NewFixedSliceWriter(int(init.Size()))
		if err = init.EncodeSW(sw); err != nil {
			return "", fmt.Errorf("couldn't write init to sw: %w", err)
		}

		encodedInit := base64.StdEncoding.EncodeToString(sw.Bytes())
		return encodedInit, nil
	} else {
		segmentByte, err := base64.StdEncoding.DecodeString(segmentStr)
		if err != nil {
			return "", fmt.Errorf("couldn't decode segment: %w", err)
		}

		sr := bits.NewFixedSliceReader(segmentByte)

		file, err := mp4.DecodeFileSR(sr)
		if err != nil {
			return "", fmt.Errorf("couldn't parse init: %w", err)
		}

		for _, segment := range file.Segments {

			log.Println(segment.Size())

			for _, k := range keys {

				split := strings.Split(k, ":")
				if len(split) < 2 {
					log.Println("couldn't split key:", k)
					continue
				}
				k = split[1]

				key, err := mp4.UnpackKey(k)
				if err != nil {
					log.Println("couldn't unpack key")
					continue
				}

				if err := mp4.DecryptSegment(segment, di, key); err != nil {
					log.Println("couldn't decrypt segment")
					continue
				}

				log.Println("KEY:", di)
			}

			log.Println(segment.Size())
		}

		log.Println("SEGMENT:")

		sw := bits.NewFixedSliceWriter(int(file.Size()))
		if err = file.EncodeSW(sw); err != nil {
			return "", fmt.Errorf("couldn't write segment to sw: %w", err)
		}

		encodedMedia := base64.StdEncoding.EncodeToString(sw.Bytes())
		return encodedMedia, nil
	}
}
