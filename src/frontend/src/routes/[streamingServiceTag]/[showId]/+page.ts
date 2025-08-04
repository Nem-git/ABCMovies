import type { PageLoad } from "./$types";

import type { Show } from "$lib/types";

import { getShowUrl } from "$lib/api";

export const load: PageLoad = async ({ fetch, parent }) => {
    let { streamingServiceTag, showId } = await parent();

    let showPromise: Promise<Show> = fetch(
        getShowUrl(streamingServiceTag, showId),
    ).then((r: Response) => r.json());

    return {
        show: await showPromise,
        season: (await showPromise).seasons[0],
    };
};
