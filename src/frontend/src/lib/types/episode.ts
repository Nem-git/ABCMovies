export type EpisodeRequest = {
    ServiceTag: string;
    ShowID: string;
    SeasonNumber: number;
    EpisodeNumber: number;
};

export type Episode = {
    streams: EpisodeStream[];
    backdropURL: string;
    number: number;
    name: string;
    originalName: string;
    overview: string;
    posterURL: string;
    mediaType: string;
    originalLanguage: string;
    length: number;
    cast: string[];
    directors: string[];
    firstAirDate: string;
    originCountry: string;
    availabilityStatus: string;
    described: boolean;
    videoTracks: EpisodeVideoTrack[];
    audioTracks: EpisodeAudioTrack[];
    textTracks: EpisodeTextTrack[];
    cuePoints: EpisodeCuePoint[];
};

export type NextEpisode = {
    showID: string;
    seasonNumber: number;
};

export type EpisodeStream = {
    type: string;
    url: string;
};

export type EpisodeVideoTrack = {
    name: string;
    height: number;
    width: number;
    bitrate: number;
};

export type EpisodeAudioTrack = {
    code: string;
    name: string;
    originalName: string;
};

export type EpisodeTextTrack = {
    type: string;
    name: string;
    language: string;
    trackURL: string;
};

export type EpisodeCuePoint = {
    name: string;
    start: number;
    end: number;
};
