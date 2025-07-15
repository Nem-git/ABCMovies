export type Show = {
    id: string;
    title: string;
    year: number;
    fullDescription: string;
    shortDescription: string;
    imageCard: string;
    imageBackground: string;
    provider: string;
    seasons: Season[];
};

export type Season = {
    id: string;
    title: string;
    number: number;
    fullDescription: string;
    shortDescription: string;
    provider: string;
    episodes: Episode[];
};

export type Episode = {
    id: string;
    title: string;
    number: number;
    fullDescription: string;
    shortDescription: string;
    imageCard: string;
    provider: string;
    url: string;
};
