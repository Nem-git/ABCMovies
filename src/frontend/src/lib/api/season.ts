import { API_URL } from "./config";
import type { Season } from "./config";

export const getSeason = async (nodeUrl: string): Promise<Season> => {
	let url = API_URL + nodeUrl;
	let resp = await fetch(url);
	if (resp.ok) {
		return resp.json();
	} else {
		console.log("Couldn't fetch season from url: " + url);
		throw new Error("Couldn't fetch season from url: " + url);
	}
};
