import type { PageLoad } from "./$types";

import type { StreamingService } from "$lib/types";

import { getStreamingServiceUrl } from "$lib/api";

export const load: PageLoad = async ({ fetch, parent }) => {
    let { streamingServiceTag } = await parent();

    let streamingServicePromise = fetch(
        getStreamingServiceUrl(streamingServiceTag),
    ).then((r: Response): Promise<StreamingService> => r.json());

    return {
        streamingService: await streamingServicePromise,
    };
};
