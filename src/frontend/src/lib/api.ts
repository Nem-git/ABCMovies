import { API_URL } from "./constants";
import type { Show } from "./types";
import type { Season } from "./types";
import type { Episode } from "./types";

export const request = async (path: string): Promise<any> => {
    let resp = fetch(API_URL + path);
    if ((await resp).ok) {
        return (await resp).json();
    }

    console.error(`Got http error ${(await resp).status} from ${path}`);
    return "";
};

export const getSearchResults = async (query: string): Promise<Show[]> => {
    let url = `/search/${encodeURI(query)}`;
    return request(url);
};

export const getShow = async (
    streamingService: string,
    show: string,
): Promise<Show> => {
    let url = "/" + [streamingService, show].join("/");
    return request(url);
};

export const getSeason = async (
    streamingService: string,
    show: string,
    season: string,
): Promise<Season> => {
    let url = "/" + [streamingService, show, season].join("/");
    return request(url);
};

export const getEpisode = async (
    streamingService: string,
    show: string,
    season: string,
    episode: string,
): Promise<Episode> => {
    let url = "/" + [streamingService, show, season, episode].join("/");
    return request(url);
};
