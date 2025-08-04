import type { LayoutParams, PageLoad } from "./$types";
import type { Episode } from "$lib/types";
import { getEpisodeUrl } from "$lib/api";

export const load: PageLoad = async ({ fetch, parent }) => {
    let {
        streamingServiceTag,
        showId,
        seasonNumber,
        episodeNumber,
    }: LayoutParams = await parent();

    let url: string = getEpisodeUrl(
        streamingServiceTag,
        showId,
        seasonNumber,
        episodeNumber,
    );
    let episodePromise: Promise<Episode> = fetch(url).then((r) => r.json());

    return {
        episode: await episodePromise,
    };
};
