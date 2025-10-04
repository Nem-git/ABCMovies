package requests

import "errors"

var ErrEmptyPageID = errors.New("page ID invalid")
var ErrEmptyCategoryID = errors.New("category ID invalid")

var ErrInvalidWidevinePSSH = errors.New("pssh invalid")
var ErrEmptyWidevineLicenseURL = errors.New("license server URL empty")
var ErrEmptyWidevinePlaylistURL = errors.New("playlist URL empty")
var ErrEmptyKeysWidevine = errors.New("no decryption keys provided")

var ErrEmptyDashManifestURL = errors.New("dash manifest url empty")
var ErrEmptyDashManifestContent = errors.New("dash manifest content empty")

var ErrEmptySearchQuery = errors.New("search query empty")

var ErrInvalidServiceTag = errors.New("service tag empty")
var ErrEmptyServiceTag = errors.New("service tag invalid")
var ErrEmptyShowID = errors.New("show ID empty")
var ErrEmptySeasonNumber = errors.New("season number empty")
var ErrInvalidSeasonNumber = errors.New("season number invalid")
var ErrEmptyEpisodeNumber = errors.New("episode number empty")
var ErrInvalidEpisodeNumber = errors.New("episode number invalid")
