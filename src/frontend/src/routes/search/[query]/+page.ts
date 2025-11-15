import { getSearchURL } from "$lib/api/search";
import type { Search, SearchRequest } from "$lib/types/search";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ fetch, params }) => {
    const request: SearchRequest = {
        Query: params.query,
    };

    const search: Search = await fetch(getSearchURL(request)).then((r) =>
        r.json(),
    );

    return {
        search: search,
    };
};
