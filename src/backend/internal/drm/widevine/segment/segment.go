package segment

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/storage/cache/connector"
	"github.com/nem-git/abcmovies/internal/storage/cache/controller"
	widevineController "github.com/nem-git/abcmovies/internal/storage/cache/controller/widevine"
	widevineRepo "github.com/nem-git/abcmovies/internal/storage/cache/repository/widevine"
)

// TODO: Fix the decryption
func Get(initByte []byte, segmentByte []byte, strKeys []string, wantInit bool, dbID string) ([]byte, error) {

	// Return decrypted init
	if wantInit {
		initByte, err := GetUsingDB(dbID)

		if err == nil {
			initMP4, err := decodeByteSegment(initByte)
			if err != nil {
				return nil, err
			}

			_, err = mp4.DecryptInit(initMP4.Init)
			if err != nil {
				return nil, fmt.Errorf("couldn't clean init: %w", err)
			}

			return encodeByteInit(initMP4.Init)
		}
	}

	initMP4, err := decodeByteSegment(initByte)
	if err != nil {
		return nil, fmt.Errorf("couldn't decode init: %w", err)
	}

	segmentMP4, err := decodeByteSegment(segmentByte)
	if err != nil {
		return nil, fmt.Errorf("couldn't decode segment: %w", err)
	}

	init, err := getInit(initMP4, segmentMP4)
	if err != nil {
		return nil, fmt.Errorf("couldn't extract init: %w", err)
	}

	// Deep copy
	encryptedInit, err := copyInit(init)
	if err != nil {
		return nil, fmt.Errorf("couldn't copy init: %w", err)
	}

	decryptInfo, err := mp4.DecryptInit(init)
	if err != nil {
		return nil, fmt.Errorf("couldn't clean init: %w", err)
	}

	// Return decrypted init
	if wantInit {
		encodedEncryptedInit, err := encodeBase64Init(encryptedInit)
		if err != nil {
			return nil, fmt.Errorf("couldn't base64 encode init: %w", err)
		}

		if err := SaveUsingDB(dbID, encodedEncryptedInit); err != nil {
			return nil, fmt.Errorf("couldn't save base64 encode init: %w", err)
		}

		t, _ := encodeByteInit(encryptedInit)
		os.WriteFile("encrypted_init.mp4", t, 0644)

		t, _ = encodeByteInit(init)
		os.WriteFile("decrypted_init.mp4", t, 0644)

		return encodeByteInit(init)
	}

	segmentMP4.Init = encryptedInit

	t, _ := encodeByteSegment(segmentMP4)
	log.Println(segmentMP4.Size())

	os.WriteFile("encrypted_segment.mp4", t, 0644)

	w := new(bytes.Buffer)

	if err = init.Encode(w); err != nil {
		return nil, fmt.Errorf("couldn't encode init: %w", err)
	}

	for _, k := range strKeys {
		split := strings.Split(k, ":")
		if len(split) != 2 {
			log.Println("couldn't split key:", k)
			continue
		}

		k = split[0]

		keyByte, err := mp4.UnpackKey(k)
		if err != nil {
			log.Println("couldn't unpack key:", k)
			continue
		}

		// widevine.DecryptMP4Auto()

		// Decode segments
		for _, seg := range segmentMP4.Segments {
			if err = mp4.DecryptSegment(seg, decryptInfo, keyByte); err != nil {
				if err.Error() == "no senc box in traf" {
					// No SENC box, skip decryption for this segment as samples can have
					// unencrypted segments followed by encrypted segments. See:
					// https://github.com/iyear/gowidevine/pull/26#issuecomment-2385960551
					err = nil
				} else {
					log.Println("couldn't decrypt segment")
					return nil, err
				}
			}
		}
	}

	for _, seg := range segmentMP4.Segments {
		if err = seg.Encode(w); err != nil {
			return nil, fmt.Errorf("couldn't encode segment: %w", err)
		}
		log.Println()
	}

	os.WriteFile("decrypted_segment.mp4", w.Bytes(), 0644)

	// Return decrypted segment
	return w.Bytes(), nil
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
