import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ fetch, parent }) => {
    let { streamingServiceTag } = await parent();

    // let url: string = getStreamingServiceUrl(streamingService.tag);
    // let streamingServicePromise: Promise<StreamingService> = fetch(url).then(r => r.json());

    // return {
    //     streamingService: await streamingServicePromise,
    // };
};
