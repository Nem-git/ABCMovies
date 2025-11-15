import { getServicesURL } from "$lib/api/service";
import type { Services } from "$lib/types/service";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ fetch }) => {
    const services: Services = await fetch(getServicesURL()).then((r) =>
        r.json(),
    );

    return {
        services: services,
    };
};
