import type { PageLoad } from "./$types";

export const load: PageLoad = async () => {
    return {
        search: {
            query: "",
            showCount: 0,
            shows: [],
        },
    };
};
