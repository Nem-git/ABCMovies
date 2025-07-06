import type { Show } from "./config";
import { API_URL } from "./config";

export const getSearchResults = async (query: string): Promise<Show[]> => {
	let url = `${API_URL}/search/${encodeURI(query)}`;
	let resp = await fetch(url);
	console.log(url);
	if (resp.ok) {
		return resp.json();
	} else {
		throw new Error(`Couldn't fetch search results from url: ${url}`);
	}
};
