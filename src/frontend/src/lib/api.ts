import { API_URL, INTERNAL_API_URL } from "./constants";

import { browser } from "$app/environment";

const getApiUrl = () => {
    return browser ? API_URL : INTERNAL_API_URL;
};

export const getSearchResultsUrl = (query: string): string => {
    return `${getApiUrl()}search/${encodeURI(query)}`;
};

// export const getStreamingService = async (
//     streamingService: string
// ): Promise<StreamingService> => {
//     let url = "/" + streamingService;
//     return request(url);
// };

export const getShowUrl = (
    streamingService: string,
    showId: string,
): string => {
    return getApiUrl() + [streamingService, showId].join("/");
};

export const getSeasonUrl = (
    streamingService: string,
    showId: string,
    seasonNumber: number,
): string => {
    return getApiUrl() + [streamingService, showId, seasonNumber].join("/");
};

export const getEpisodeUrl = (
    streamingService: string,
    showId: string,
    seasonNumber: number,
    episodeNumber: number,
): string => {
    return (
        getApiUrl() +
        [streamingService, showId, seasonNumber, episodeNumber].join("/")
    );
};
