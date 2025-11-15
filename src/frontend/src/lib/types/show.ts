import type { Season } from "./season";

export type ShowRequest = {
    ServiceTag: string;
    ShowID: string;
};

export type Show = {
    adult: boolean;
    backdropURL: string;
    id: string;
    seasonCount: number;
    name: string;
    originalName: string;
    overview: string;
    posterURL: string;
    mediaType: string;
    originalLanguage: string;
    genres: ShowGenre[];
    cast: string[];
    directors: string[];
    firstAirDate: string;
    originCountry: string;
    availabilityStatus: string;
    seasons: Season[];
};

export type ShowGenre = {
    id: string;
    name: string;
};
