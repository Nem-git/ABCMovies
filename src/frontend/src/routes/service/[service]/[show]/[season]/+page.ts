import { getSeasonURL } from "$lib/api/season";
import type { Season, SeasonRequest } from "$lib/types/season";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ fetch, params }) => {
    const request: SeasonRequest = {
        ServiceTag: params.service,
        ShowID: params.show,
        SeasonNumber: parseInt(params.season),
    };

    const season: Season = await fetch(getSeasonURL(request)).then((r) =>
        r.json(),
    );

    return {
        season: season,
    };
};
