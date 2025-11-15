import { getShowURL } from "$lib/api/show";
import type { Show, ShowRequest } from "$lib/types/show";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ fetch, params }) => {
    const request: ShowRequest = {
        ServiceTag: params.service,
        ShowID: params.show,
    };

    const show: Show = await fetch(getShowURL(request)).then((r) => r.json());

    return {
        show: show,
    };
};
