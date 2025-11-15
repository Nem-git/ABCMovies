import { getServiceURL } from "$lib/api/service";
import type { Service, ServiceRequest } from "$lib/types/service";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ fetch, params }) => {
    const request: ServiceRequest = {
        ServiceTag: params.service,
    };

    const service: Service = await fetch(getServiceURL(request)).then((r) =>
        r.json(),
    );

    return {
        service: service,
    };
};
