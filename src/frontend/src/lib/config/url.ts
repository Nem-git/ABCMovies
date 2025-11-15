import { browser } from "$app/environment";

export const EXTERNAL_API_URL = "http://localhost:8080/api";
export const INTERNAL_API_URL = "http://localhost:8080/api";

export const getAPIURL = () => {
    return browser ? EXTERNAL_API_URL : INTERNAL_API_URL;
};
