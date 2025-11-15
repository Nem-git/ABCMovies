import { getEpisodeURL } from "$lib/api/episode";
import type { Episode, EpisodeRequest } from "$lib/types/episode";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ fetch, params }) => {
    const request: EpisodeRequest = {
        ServiceTag: params.service,
        ShowID: params.show,
        SeasonNumber: parseInt(params.season),
        EpisodeNumber: parseInt(params.episode),
    };

    const episode: Episode = await fetch(getEpisodeURL(request)).then((r) =>
        r.json(),
    );

    return {
        episode: episode,
    };
};
