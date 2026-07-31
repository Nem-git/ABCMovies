package types

import "time"

type EpisodeExternalResponse struct {
	ContentID                              string    `json:"content.id"`
	ContentMediaID                         string    `json:"content.mediaId"`
	ContentFriendlyID                      string    `json:"content.friendlyId"`
	ContentHierarchyName                   string    `json:"content.hierarchyName"`
	ContentSeason                          string    `json:"content.season"`
	ContentEpisode                         string    `json:"content.episode"`
	ContentEpisodeOnAir                    string    `json:"content.episodeOnAir"`
	ContentTitle                           string    `json:"content.title"`
	ContentLength                          string    `json:"content.length"`
	ContentIsFullContent                   string    `json:"content.isFullContent"`
	ContentProducerName                    string    `json:"content.producerName"`
	ContentCms                             string    `json:"content.cms"`
	ContentStreamType                      string    `json:"content.streamType"`
	ContentTotalSegments                   string    `json:"content.totalSegments"`
	ContentComscoreMediaType               string    `json:"content.comscoreMediaType"`
	ContentComscoreMediaFormat             string    `json:"content.comscoreMediaFormat"`
	ContentComscoreProgramTitle            string    `json:"content.comscoreProgramTitle"`
	ContentComscoreEpisodeID               string    `json:"content.comscoreEpisodeId"`
	ContentComscoreEpisodeSeasonNumber     string    `json:"content.comscoreEpisodeSeasonNumber"`
	ContentComscoreEpisodeNumber           string    `json:"content.comscoreEpisodeNumber"`
	ContentComscoreC4                      string    `json:"content.comscoreC4"`
	ContentComscoreKeyword                 string    `json:"content.comscoreKeyword"`
	ContentCmfContractNumber               string    `json:"content.cmfContractNumber"`
	ContentCollectionIds                   string    `json:"content.collectionIds"`
	ContentCollectionNames                 string    `json:"content.collectionNames"`
	ContentSubjectsPrincipal               string    `json:"content.subjects.principal"`
	ContentSubjectsLevel1                  string    `json:"content.subjects.level1"`
	ContentSubjectsLevel2                  string    `json:"content.subjects.level2"`
	ContentAppartenanceScoop               string    `json:"content.appartenanceScoop"`
	ContentEtiquetteScoop                  string    `json:"content.etiquetteScoop"`
	ShowID                                 string    `json:"show.id"`
	ShowCodeName                           string    `json:"show.codeName"`
	ShowName                               string    `json:"show.name"`
	ShowProgramIDOnAir                     string    `json:"show.programIdOnAir"`
	ShowAudiencePrincipal                  string    `json:"show.audience.principal"`
	ShowAudienceAdditional                 string    `json:"show.audience.additional"`
	ShowIsFamilyFriendly                   string    `json:"show.isFamilyFriendly"`
	ShowGenresPrincipal                    string    `json:"show.genres.principal"`
	ShowGenresLevel1                       string    `json:"show.genres.level1"`
	ShowGenresLevel2                       string    `json:"show.genres.level2"`
	ShowSubjectsPrincipal                  string    `json:"show.subjects.principal"`
	ShowSubjectsLevel1                     string    `json:"show.subjects.level1"`
	ShowSubjectsLevel2                     string    `json:"show.subjects.level2"`
	ShowFormat                             string    `json:"show.format"`
	ShowType                               string    `json:"show.type"`
	ShowSponsors                           string    `json:"show.sponsors"`
	ShowTrackForAds                        string    `json:"show.trackForAds"`
	ShowContentType                        string    `json:"show.contentType"`
	EventID                                string    `json:"event.id"`
	EventName                              string    `json:"event.name"`
	DistributionChannelsOttPrincipal       string    `json:"distribution.channels.ott.principal"`
	DistributionChannelsOttAdditional      string    `json:"distribution.channels.ott.additional"`
	DistributionChannelsStation            string    `json:"distribution.channels.station"`
	DistributionChannelsNumerisCodeStation string    `json:"distribution.channels.numerisCodeStation"`
	DistributionLastBroadcastDatetime      string    `json:"distribution.lastBroadcastDatetime"`
	DistributionFirstWebcastDatetime       time.Time `json:"distribution.firstWebcastDatetime"`
	DistributionMode                       string    `json:"distribution.mode"`
	DistributionIs7DaysCatchup             string    `json:"distribution.is7DaysCatchup"`
	DistributionTier                       string    `json:"distribution.tier"`
	DistributionComscoreDistributionModel  string    `json:"distribution.comscoreDistributionModel"`
	ServerDomain                           string    `json:"server.domain"`
	ServerDatetime                         time.Time `json:"server.datetime"`
}
