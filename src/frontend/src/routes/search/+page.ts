import type { PageLoad } from "./$types";
import { getSearchResultsUrl } from "$lib/api";
import type { Show } from "$lib/types";

export const load: PageLoad = async ({ fetch, url }) => {
    let query = url.searchParams.get("q");

    let searchResults: Array<Show> = new Array<Show>();

    if (query) {
        searchResults = await fetch(getSearchResultsUrl(query)).then((r) =>
            r.json(),
        );
    }

    return {
        searchResults: searchResults,
        query: query ?? "",
    };
};
