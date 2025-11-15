import { getAPIURL } from "$lib/config/url";
import type { ServiceRequest } from "$lib/types/service";

export const getServiceURL = (request: ServiceRequest): string => {
    return `${getAPIURL()}/service/${request.ServiceTag}`;
};

export const getServicesURL = (): string => {
    return `${getAPIURL()}/service`;
};
