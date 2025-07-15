import type { PageLoad } from "./$types";
import type { Episode } from "$lib/types";
import { getEpisodeUrl } from "$lib/api";

export const load: PageLoad = async ({ fetch, parent }) => {
    let { streamingServiceTag, showId, seasonId, episodeId } = await parent();

    let url: string = getEpisodeUrl(
        streamingServiceTag,
        showId,
        seasonId,
        episodeId,
    );
    let episodePromise: Promise<Episode> = fetch(url).then((r) => r.json());

    return {
        episodeId: episodeId,
        episode: await episodePromise,
    };
};
