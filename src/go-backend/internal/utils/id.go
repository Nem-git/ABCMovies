package utils

import (
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/http/model"
)

func GetUniqueID() string {
	return uuid.New().String()
}

func GetUniqueStreamPlaylistID(stream model.StreamRequest) string {
	return strings.Join([]string{
		stream.ServiceTag,
		stream.ShowID,
		strconv.Itoa(stream.SeasonNumber),
		strconv.Itoa(stream.EpisodeNumber),
		stream.StreamType,
	}, config.STREAM_ID_DB_SEPARATOR)
}

func GetUniqueStreamWidevinePSSHID(stream model.StreamRequest) string {
	return strings.Join([]string{
		stream.ServiceTag,
		stream.ShowID,
		strconv.Itoa(stream.SeasonNumber),
		strconv.Itoa(stream.EpisodeNumber),
		stream.StreamType,
		config.STREAM_ID_DB_PSSH_SUFFIX,
	}, config.STREAM_ID_DB_SEPARATOR)
}

func GetUniqueStreamWidevineKeysID(stream model.StreamRequest) string {
	return strings.Join([]string{
		stream.ServiceTag,
		stream.ShowID,
		strconv.Itoa(stream.SeasonNumber),
		strconv.Itoa(stream.EpisodeNumber),
		stream.StreamType,
		config.STREAM_ID_DB_WIDEVINE_KEYS_SUFFIX,
	}, config.STREAM_ID_DB_SEPARATOR)
}
