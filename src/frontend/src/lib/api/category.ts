import { getAPIURL } from "$lib/config/url";
import type {
    CategoryRequest,
    ServiceCategoriesRequest,
} from "$lib/types/category";

export const getCategoryURL = (request: CategoryRequest): string => {
    return `${getAPIURL()}/category/${request.ServiceTag}/${request.CategoryID}`;
};

export const getServiceCategoriesURL = (
    request: ServiceCategoriesRequest,
): string => {
    return `${getAPIURL()}/category/${request.ServiceTag}`;
};

export const getCategoriesURL = (): string => {
    return `${getAPIURL()}/category`;
};
