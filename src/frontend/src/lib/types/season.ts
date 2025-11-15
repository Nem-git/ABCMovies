import type { Episode } from "./episode";

export type SeasonRequest = {
    ServiceTag: string;
    ShowID: string;
    SeasonNumber: number;
};

export type Season = {
    backdropURL: string;
    number: number;
    episodeCount: number;
    name: string;
    originalName: string;
    overview: string;
    posterURL: string;
    firstAirDate: string;
    availabilityStatus: string;
    episodes: Episode[];
};
