import { API_URL } from "./constants";

export const getSearchResultsUrl = (query: string): string => {
    return `${API_URL}search/${encodeURI(query)}`;
};

// export const getStreamingService = async (
//     streamingService: string
// ): Promise<StreamingService> => {
//     let url = "/" + streamingService;
//     return request(url);
// };

export const getShowUrl = (streamingService: string, show: string): string => {
    return API_URL + [streamingService, show].join("/");
};

export const getSeasonUrl = (
    streamingService: string,
    show: string,
    season: string,
): string => {
    return API_URL + [streamingService, show, season].join("/");
};

export const getEpisodeUrl = (
    streamingService: string,
    show: string,
    season: string,
    episode: string,
): string => {
    return API_URL + [streamingService, show, season, episode].join("/");
};
