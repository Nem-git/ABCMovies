package segment

import (
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

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

func decodeByteSegment(segment []byte) (*mp4.File, error) {
	sr := bits.NewFixedSliceReader(segment)

	segmentMP4, err := mp4.DecodeFileSR(sr)
	if err != nil {
		return nil, fmt.Errorf("couldn't decode init: %w", err)
	}

	return segmentMP4, nil
}

func decodeBase64Segment(segment string) (*mp4.File, error) {

	segmentByte, err := base64.StdEncoding.DecodeString(segment)
	if err != nil {
		return nil, fmt.Errorf("couldn't base64 decode segment")
	}

	segmentMP4, err := decodeByteSegment(segmentByte)
	if err != nil {
		return nil, err
	}

	return segmentMP4, nil
}

func encodeByteInit(init *mp4.InitSegment) ([]byte, error) {

	sw := bits.NewFixedSliceWriter(int(init.Size()))

	if err := init.EncodeSW(sw); err != nil {
		return nil, errors.New("couldn't encode mp4 init segment")
	}

	return sw.Bytes(), nil
}

func encodeBase64Init(init *mp4.InitSegment) (string, error) {

	initByte, err := encodeByteInit(init)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(initByte), nil
}

func encodeByteSegment(segment *mp4.File) ([]byte, error) {

	sw := bits.NewFixedSliceWriter(int(segment.Size()))

	if err := segment.EncodeSW(sw); err != nil {
		return nil, errors.New("couldn't encode mp4 segment")
	}

	return sw.Bytes(), nil
}

func encodeBase64Segment(segment *mp4.File) (string, error) {

	segmentByte, err := encodeByteSegment(segment)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(segmentByte), nil
}
