import { API_URL } from "./config";
import type { Show } from "./config";

export const getShow = async (
	streamingService: string,
	show: string,
): Promise<Show> => {
	let url = [API_URL, streamingService, show].join("/");
	let resp = await fetch(url);
	if (resp.ok) {
		return resp.json();
	} else {
		throw new Error(`Couldn't fetch show from url: ${url}`);
	}
};
