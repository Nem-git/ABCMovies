import { getAPIURL } from "$lib/config/url";
import type { SearchRequest, ServiceSearchRequest } from "$lib/types/search";

export const getSearchURL = (request: SearchRequest) => {
    return `${getAPIURL()}/search/${encodeURIComponent(request.Query)}`;
};

export const getServiceSearchURL = (request: ServiceSearchRequest) => {
    return `${getAPIURL()}/search/${request.ServiceTag}/${encodeURIComponent(request.Query)}`;
};
