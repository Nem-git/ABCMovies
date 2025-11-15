import { getCategoryURL } from "$lib/api/category";
import type { Category, CategoryRequest } from "$lib/types/category";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({ fetch, params }) => {
    const request: CategoryRequest = {
        ServiceTag: params.service,
        CategoryID: params.category,
    };

    const category: Category = await fetch(getCategoryURL(request)).then((r) =>
        r.json(),
    );

    return {
        category: category,
    };
};
