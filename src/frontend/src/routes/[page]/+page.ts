import { getPageURL } from "$lib/api/page";
import type { Page, PageRequest } from "$lib/types/page";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ fetch, params }) => {
    const request: PageRequest = {
        PageID: params.page,
    };

    const page: Page = await fetch(getPageURL(request)).then((r) => r.json());

    return {
        page: page,
    };
};
