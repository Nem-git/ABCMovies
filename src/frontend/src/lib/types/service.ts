export type ServiceRequest = {
    ServiceTag: string;
};

export type Service = {
    backdropURL: string;
    id: string;
    name: string;
    originalName: string;
    overview: string;
    posterURL: string;
    mediaTypes: string[];
    originalLanguage: string;
    homePage: string;
    originCountry: string;
    availabilityStatus: string;
};

export type Services = {
    serviceCount: number;
    services: Service[];
};
