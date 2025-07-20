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

export const getShowUrl = (streamingService: string, show: string): string => {
    return getApiUrl() + [streamingService, show].join("/");
};

export const getSeasonUrl = (
    streamingService: string,
    show: string,
    season: string,
): string => {
    return getApiUrl() + [streamingService, show, season].join("/");
};

export const getEpisodeUrl = (
    streamingService: string,
    show: string,
    season: string,
    episode: string,
): string => {
    return getApiUrl() + [streamingService, show, season, episode].join("/");
};
