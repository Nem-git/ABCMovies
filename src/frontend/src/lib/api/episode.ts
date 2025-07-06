import { API_URL } from "./config";
import type { Episode } from "./config";

export const getEpisode = async (
	streamingService: string,
	show: string,
	season: string,
	episode: string,
): Promise<Episode> => {
	let url = `${API_URL}/${[streamingService, show, season, episode].join("/")}`;
	console.log(url);
	let resp = await fetch(url);
	if (resp.ok) {
		return resp.json();
	} else {
		throw new Error("Couldn't fetch episode from url: " + url);
	}
};
