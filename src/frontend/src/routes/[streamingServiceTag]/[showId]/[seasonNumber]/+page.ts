import type { PageLoad } from "./$types";

import type { Show } from "$lib/types";
import type { Season } from "$lib/types";

import { getSeasonUrl, getShowUrl } from "$lib/api";

export const load: PageLoad = async ({ fetch, parent }) => {
    let { streamingServiceTag, showId, seasonNumber } = await parent();

    let showPromise = fetch(getShowUrl(streamingServiceTag, showId)).then(
        (r: Response): Promise<Show> => r.json(),
    );

    let seasonPromise = fetch(
        getSeasonUrl(streamingServiceTag, showId, seasonNumber),
    ).then((r: Response): Promise<Season> => r.json());

    return {
        show: await showPromise,
        season: await seasonPromise,
    };
};
