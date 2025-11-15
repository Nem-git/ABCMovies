import { getAPIURL } from "$lib/config/url";
import type { PageRequest } from "$lib/types/page";

export const getPageURL = (request: PageRequest): string => {
    return `${getAPIURL()}/page/${request.PageID}`;
};

export const getPagesURL = (): string => {
    return `${getAPIURL()}/page`;
};
