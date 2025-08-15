type AssociativeArray<T = unknown> = { [key: string]: T | undefined } | T[];

export type StreamingService = {
    /**
     * Streaming service's name
     */
    name: string;
    /**
     * Streaming service's abreviation (EX: DSNP)
     */
    tag: string;
    /**
     * Short form description of the streaming service
     */
    shortDescription: string;
    /**
     * Long form description of the streaming service
     */
    fullDescription: string;
    /**
     * Card image URL
     */
    imageCard: string;
    /**
     * Background image URL
     */
    imageBackground: string;
};

export type Show = {
    /**
     * Show unique identifier (In the streaming service)
     */
    id: string;
    /**
     * Show title (Ex: La petite vie)
     */
    title: string;
    /**
     * Release year
     */
    year: number;
    /**
     * Show long form description
     */
    fullDescription: string;
    /**
     * Show short form description
     */
    shortDescription: string;
    /**
     * Card image URL
     */
    imageCard: string;
    /**
     * Background image URL
     */
    imageBackground: string;
    /**
     * The streaming service's tag
     */
    streamingServiceTag: string;
    /**
     * Seasons in the show
     */
    seasons: Season[];
};

export type Season = {
    /**
     * Season unique identifier (In the streaming service)
     */
    id: string;
    /**
     * Season title (Ex: Le voyage à Plattsburg)
     */
    title: string;
    /**
     * Season number
     */
    number: number;
    /**
     * Season long form description
     */
    fullDescription: string;
    /**
     * Season short form description
     */
    shortDescription: string;
    /**
     * Show unique identifier (In the streaming service)
     */
    showId: string;
    /**
     * The streaming service's tag
     */
    streamingServiceTag: string;
    /**
     * Entirety of episodes in the season
     */
    episodes: Episode[];
};

export type Episode = {
    /**
     * Episode's unique identifier (In the streaming service)
     */
    id: string;
    /**
     * Episode's title (Ex: Le voyage à Plattsburg)
     */
    title: string;
    /**
     * Episode number
     */
    number: number;
    /**
     * Episode's long form description
     */
    fullDescription: string;
    /**
     * Episode's short form description
     */
    shortDescription: string;
    /**
     * Card image URL
     */
    imageCard: string;
    /**
     * Download link. Local download (PHP backend)
     */
    url: string;
    /**
     * Headers required to use the download link
     */
    urlHeaders: AssociativeArray<string>;
    /**
     * The chosen streaming technology using settings in constants
     */
    streamingTechnology: null; // Don't know how to represent it
    /**
     * Wether or not the episode's video is DRM-protected
     */
    containsDrm: boolean;
    /**
     * Season Number
     */
    seasonNumber: number;
    /**
     * Show unique identifier (In the streaming service)
     */
    showId: string;
    /**
     * The streaming service's tag
     */
    streamingServiceTag: string;
};
