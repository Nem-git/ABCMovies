package segment

import (
	"fmt"
	"log"
	"strings"

	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/storage/cache/connector"
	"github.com/nem-git/abcmovies/internal/storage/cache/controller"
	widevineController "github.com/nem-git/abcmovies/internal/storage/cache/controller/widevine"
	widevineRepo "github.com/nem-git/abcmovies/internal/storage/cache/repository/widevine"
)

func Get(initByte []byte, segmentByte []byte, keys []string, wantInit bool, dbID string) ([]byte, error) {

	if wantInit {
		segment, err := GetUsingDB(dbID)
		if err == nil {
			return segment, nil
		}
	}

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

	encryptedInit := *init

	decryptInfo, err := mp4.DecryptInit(init)
	if err != nil {
		return nil, fmt.Errorf("couldn't clean init: %w", err)
	}

	// Return decrypted init
	if wantInit {
		encodedInit, err := encodeBase64Init(&encryptedInit)
		if err != nil {
			return nil, err
		}

		if err := SaveUsingDB(dbID, encodedInit); err != nil {
			return nil, err
		}

		return encodeByteInit(init)
	}

	// segmentMP4.Init = init

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

func GetUsingDB(dbID string) ([]byte, error) {
	controller := connectToDB()

	segment, err := controller.ReadSingle(dbID)
	if err != nil {
		return nil, err
	}

	return decodeBase64SegmentToByte(segment)
}

func SaveUsingDB(dbID string, content string) error {
	controller := connectToDB()

	if err := controller.Create(dbID, content); err != nil {
		return err
	}

	return nil
}

func connectToDB() controller.CacheController {
	conn := connector.NewRedisConnector(connector.ConnectionDetails{
		Address:  config.TEMP_REDIS_ADDRESS,
		User:     config.TEMP_REDIS_USER,
		Password: config.TEMP_REDIS_PASSWORD,
		DB:       config.TEMP_REDIS_DB,
	})
	repo := widevineRepo.NewSegmentRepository(conn)
	controller := widevineController.NewSegmentController(repo)

	return controller
}
