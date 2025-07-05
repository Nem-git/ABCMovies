import { API_URL } from "./config";
import type { Episode } from "./config";

export const getEpisode = async (nodeUrl: string): Promise<Episode> => {
	let url = API_URL + nodeUrl;
	let resp = await fetch(url);
	if (resp.ok) {
		return resp.json();
	} else {
		console.log("Couldn't fetch episode from url: " + url);
		throw new Error("Couldn't fetch episode from url: " + url);
	}
};
