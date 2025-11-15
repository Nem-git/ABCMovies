import { getAPIURL } from "$lib/config/url";
import type { SeasonRequest } from "$lib/types/season";

export const getSeasonURL = (request: SeasonRequest): string => {
    return `${getAPIURL()}/service/${request.ServiceTag}/${request.ShowID}/${request.SeasonNumber}`;
};
