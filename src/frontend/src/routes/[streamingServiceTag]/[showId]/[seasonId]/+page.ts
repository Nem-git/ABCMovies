import type { PageLoad } from "./$types";

import type { Show } from "$lib/types";
import type { Season } from "$lib/types";

import { getSeasonUrl, getShowUrl } from "$lib/api";

export const load: PageLoad = async ({ parent, fetch }) => {
    let { streamingServiceTag, showId, seasonId } = await parent();

    let showPromise: Promise<Show> = fetch(
        getShowUrl(streamingServiceTag, showId),
    ).then((r) => r.json());

    let seasonPromise: Promise<Season> = fetch(
        getSeasonUrl(streamingServiceTag, showId, seasonId),
    ).then((r) => r.json());

    return {
        showId: showId,
        show: await showPromise,
        seasonId: seasonId,
        season: await seasonPromise,
    };
};
