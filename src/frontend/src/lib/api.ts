import { API_URL, INTERNAL_API_URL } from "./constants";

import { browser } from "$app/environment";

const getApiUrl = () => {
    return browser ? API_URL : INTERNAL_API_URL;
};

export const getSearchResultsUrl = (query: string): string => {
    return `${getApiUrl()}search/${encodeURI(query)}`;
};

export const getStreamingServiceUrl = (streamingServiceTag: string): string => {
    return getApiUrl() + streamingServiceTag;
};

export const getShowUrl = (
    streamingServiceTag: string,
    showId: string,
): string => {
    return getApiUrl() + [streamingServiceTag, showId].join("/");
};

export const getSeasonUrl = (
    streamingServiceTag: string,
    showId: string,
    seasonNumber: number,
): string => {
    return getApiUrl() + [streamingServiceTag, showId, seasonNumber].join("/");
};

export const getEpisodeUrl = (
    streamingServiceTag: string,
    showId: string,
    seasonNumber: number,
    episodeNumber: number,
): string => {
    return (
        getApiUrl() +
        [streamingServiceTag, showId, seasonNumber, episodeNumber].join("/")
    );
};
