package segment

import (
	"encoding/base64"
	"fmt"

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
		encodedInit := base64.StdEncoding.EncodeToString(sw.Bytes())
		return encodedInit, nil

	} else {

		segmentByte, err := base64.StdEncoding.DecodeString(segmentStr)
		if err != nil {
			return "", fmt.Errorf("couldn't decode segment: %w", err)
		}

		sr := bits.NewFixedSliceReader(segmentByte)

		segmentFile, err := mp4.DecodeFileSR(sr)
		if err != nil {
			return "", fmt.Errorf("couldn't parse init: %w", err)
		}

		segments := segmentFile.Segments

		file := mp4.NewFile()

		for _, segment := range segments {
			for _, k := range keys {
				key, err := mp4.UnpackKey(k)
				if err != nil {
					mp4.DecryptSegment(segment, di, key)
					file.AddMediaSegment(segment)
				}
			}
		}

		sw := bits.NewFixedSliceWriter(int(file.Size()))
		encodedMedia := base64.StdEncoding.EncodeToString(sw.Bytes())
		return encodedMedia, nil
	}
}
