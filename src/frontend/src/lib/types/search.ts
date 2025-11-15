import type { Show } from "./show";

export type SearchRequest = {
    Query: string;
};

export type ServiceSearchRequest = SearchRequest & {
    ServiceTag: string;
};

export type Search = {
    query: string;
    showCount: number;
    shows: SearchResult[];
};

export type SearchResult = Show & {
    serviceTag?: string;
};
