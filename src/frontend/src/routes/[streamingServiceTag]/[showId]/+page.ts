import type { PageLoad } from "./$types";
import { goto } from "$app/navigation";

import type { Show } from "$lib/types";

import { getShowUrl } from "$lib/api";

export const load: PageLoad = async ({ fetch, parent }) => {
    let { streamingServiceTag, showId } = await parent();

    let showPromise: Promise<Show> = fetch(
        getShowUrl(streamingServiceTag, showId),
    ).then((r) => r.json());

    return {
        show: await showPromise,
        seasonId: (await showPromise).seasons[0].number.toString(),
    };
};
