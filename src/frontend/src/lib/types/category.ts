import type { Show } from "./show";

export type CategoryRequest = {
    ServiceTag: string;
    CategoryID: string;
};

export type ServiceCategoriesRequest = {
    ServiceTag: string;
};

export type Category = {
    backdropURL: string;
    id: string;
    name: string;
    overview: string;
    posterURL: string;
    shows: Show[];
    serviceTag?: string;
};

export type Categories = {
    categoryCount: number;
    categories: Category[];
};
