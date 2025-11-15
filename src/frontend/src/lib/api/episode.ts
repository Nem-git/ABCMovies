import { getAPIURL } from "$lib/config/url";
import type { EpisodeRequest } from "$lib/types/episode";

export const getEpisodeURL = (request: EpisodeRequest): string => {
    return `${getAPIURL()}/service/${request.ServiceTag}/${request.ShowID}/${request.SeasonNumber}/${request.EpisodeNumber}`;
};

export const getNextEpisodeURL = (request: EpisodeRequest): string => {
    return `${getAPIURL()}/service/${request.ServiceTag}/${request.ShowID}/${request.SeasonNumber}/${request.EpisodeNumber}/next`;
};
