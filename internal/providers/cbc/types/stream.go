package types

import "time"

// Meta
// Example:
// https://services.radio-canada.ca/media/meta/v1/index.ashx
// params {
//   appCode: gem,
//   idMedia: 990148,
//   output: jsonObject
// }

type StreamMetaResponse struct {
	ErrorMessage   *ErrorMessage    `json:"errorMessage"`
	AvailableTechs []AvailableTechs `json:"availableTechs"`
	Metas          *Metas           `json:"Metas"`
}

type ErrorMessage struct {
	ErrorCode int    `json:"errorCode"`
	Text      string `json:"text"`
}

type AvailableTechs struct {
	Drm              []string `json:"drm"`
	Name             string   `json:"name"`
	ManifestVersions []string `json:"manifestVersions"`
}

type Metas struct {
	AppCode              string `json:"appCode"`
	IDMedia              string `json:"idMedia"`
	Progressive          string `json:"progressive"`
	FileID               string `json:"FileID"`
	ProviderID           string `json:"providerId"`
	ClosedCaption        string `json:"closedCaption"`
	ClosedCaptionRaw     string `json:"closedCaptionRaw"`
	ClosedCaptionHTML5   string `json:"closedCaptionHTML5"`
	DescribedVideo       string `json:"describedVideo"`
	EIA608ClosedCaptions string `json:"EIA608ClosedCaptions"`
	Shareable            string `json:"shareable"`
	PlancheContact       string `json:"plancheContact"`
	PlancheContactHR     string `json:"plancheContactHR"`
	AdsAlways            string `json:"adsAlways"`
	ProtectionType       string `json:"protectionType"`
	IsDrmActive          string `json:"isDrmActive"`
	ProtectionSchemes    string `json:"protectionSchemes"`
	NoFMC                string `json:"noFMC"`
	ReseauNom            string `json:"ReseauNom"`
	ReseauID             string `json:"ReseauID"`
	Bitrates             string `json:"bitrates"`
	LastModifiedDate     string `json:"lastModifiedDate"`
	AvEmission           string `json:"Av-Emission"`
	Title                string `json:"Title"`
	TitleID              string `json:"TitleID"`
	Author               string `json:"Author"` // Ex: Alex A in L'agent Jean
	Chapitres            string `json:"Chapitres"`
	ClipType             string `json:"ClipType"`
	Description          string `json:"Description"`
	CapsuleNumber        string `json:"CapsuleNumber"`
	ImagePlayerLargeA    string `json:"imagePlayerLargeA"`
	ImagePlayerNormalC   string `json:"imagePlayerNormalC"`
	ImageThumbMicroG     string `json:"imageThumbMicroG"`
	ImageThumbMoyenL     string `json:"imageThumbMoyenL"`
	ImageThumbNormalF    string `json:"imageThumbNormalF"`
	Rating               string `json:"Rating"`
	ShortDescription     string `json:"ShortDescription"`
	Emission             string `json:"Emission"`
	Thumbnail            string `json:"Thumbnail"`
	Length               string `json:"length"`
	AvDuree              string `json:"Av-Duree"`
	AvID                 string `json:"Av-Id"`
	Date                 string `json:"Date"`
	SrcAvDuree           string `json:"SrcAvDuree"`
	SrcEpisode           string `json:"SrcEpisode"`
	SrcEmission          string `json:"SrcEmission"`
	IsJeunesse           string `json:"isJeunesse"`
	RcTheme              string `json:"RcTheme"`
	RcSujet              string `json:"RcSujet"`
	RcSousTheme          string `json:"RcSousTheme"`
	SrcAvIntegrale       string `json:"SrcAvIntegrale"`
	SrcAvDiffusion       string `json:"SrcAvDiffusion"`
	IsFree               string `json:"IsFree"`
	IsReleased           string `json:"IsReleased"`
	AvNom                string `json:"Av-Nom"`
	AvGratuite           string `json:"Av-gratuite"`
	SrcSaison            string `json:"SrcSaison"`
	ZoneName             string `json:"zoneName"`
	Language             string `json:"Language"`
	Musique              string `json:"Musique"`
	SrcSource            string `json:"SrcSource"`
	Genre                string `json:"Genre"`
	PaysProduction       string `json:"paysProduction"`
	IsAvailable          string `json:"isAvailable"`
	CreditStartTime      string `json:"CreditStartTime"`
	GenericEnd           string `json:"genericEnd"`
	GeoPassed            string `json:"geoPassed"`
}

// Validation
// Example:
// https://services.radio-canada.ca/media/validation/v2/
// params {
//   appCode: gem,
//   connectionType: hd,
//   deviceType: ipad,
//   idMedia: 990148,
//   multibitrate: true,
//   output: json,
//   tech: hls,
//   manifestVersion: 2,
//   manifestType: desktop
// }

type StreamValidationResponse struct {
	URL       string    `json:"url"`
	Message   string    `json:"message"`
	ErrorCode int       `json:"errorCode"`
	Params    []Param   `json:"params"`
	Bitrates  []Bitrate `json:"bitrates"`
}

type Param struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

type Bitrate struct {
	Bitrate int       `json:"bitrate"`
	Width   int       `json:"width"`
	Height  int       `json:"height"`
	Lines   string    `json:"lines"`
	Param   time.Time `json:"param"` // just waiting for it to break to see what type to put
	Max     int       `json:"max"`
}
