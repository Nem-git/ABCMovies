import { API_URL } from "./config";
import type { Season } from "./config";

export const getSeason = async (
	streamingService: string,
	show: string,
	season: string,
): Promise<Season> => {
	let url = [API_URL, streamingService, show, season].join("/");
	let resp = await fetch(url);
	if (resp.ok) {
		return resp.json();
	} else {
		throw new Error(`Couldn't fetch season from url: " + ${url}`);
	}
};
