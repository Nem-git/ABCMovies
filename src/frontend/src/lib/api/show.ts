import { getAPIURL } from "$lib/config/url";
import type { ShowRequest } from "$lib/types/show";

export const getShowURL = (request: ShowRequest): string => {
    return `${getAPIURL()}/service/${request.ServiceTag}/${request.ShowID}`;
};
