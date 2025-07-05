import { API_URL } from "./config";
import type Show from "../lib/ShowCard.svelte";

export const getSearchResults = async (query: string): Promise<Show[]> => {
	let resp = await fetch(API_URL + `/toutv/search/${encodeURI(query)}`);
	if (resp.ok) {
		return resp.json();
	} else {
		throw new Error(
			"Couldn't fetch search results from url: " +
				API_URL +
				`/toutv/search/${query}`,
		);
	}
};
