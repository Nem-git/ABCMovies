import { API_URL } from "./config";
import type { Show } from "./config";

export const getShow = async (nodeUrl: string): Promise<Show> => {
    let url = API_URL + nodeUrl;
    let resp = await fetch(url);
    if (resp.ok) {
        return resp.json();
    }
    else {
        console.log("Couldn't fetch show from url: " + url);
        throw new Error("Couldn't fetch show from url: " + url);
    }
};