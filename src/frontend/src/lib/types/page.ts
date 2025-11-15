import type { Category } from "./category";

export type PageRequest = {
    PageID: string;
};

export type Page = {
    backdropURL: string;
    id: string;
    name: string;
    overview: string;
    posterURL: string;
    categories: Category[];
};

export type Pages = {
    pageCount: number;
    pages: Page[];
};
