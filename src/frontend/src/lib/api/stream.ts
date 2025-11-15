import { getAPIURL } from "$lib/config/url";
import type { StreamRequest } from "$lib/types/stream";

export const getStreamURL = (request: StreamRequest): string => {
    return `${getAPIURL()}/stream/${request.ServiceTag}/${request.ShowID}/${request.SeasonNumber}/${request.EpisodeNumber}/${request.StreamType}/${request.StreamFileName}`;
};
