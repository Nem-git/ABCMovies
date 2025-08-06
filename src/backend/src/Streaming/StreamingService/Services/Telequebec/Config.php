<?php

declare(strict_types=1);

namespace App\Streaming\StreamingService\Services\Telequebec;

final class Config
{
    // Pages in:
    // menus/default/option/{here}
    // 2 - En direct
    // 25 - Paramètres
    // 26 - Recherche
    // 28 - Historique
    // 29 - Se connecter
    // 30 - S'inscrire
    // 31 - Déconnexion
    // 61 - En vedette
    // 75 - Ma liste
    // 99 - Coucou
    // 101 - Télécharger l'appli
    // 102 - Squat
    // 105 - Sur demande
    // 106 - Documentaires
    // 110 - Cinéma
    // 124 - Cinéma en fête
    // 125 - Ciné-cadeau
    // 127 - Téléchargez l'application
    // 134 - La Fabrique culturelle
    // 136 - Passe-Partout
    // 137 - Séries
    // 138 - Passe-Partout -  Boutique
    // 139 - Passe-Partout - Billetterie
    // 140 - Jeunesse - petits
    // 141 - Jeunesse - grands
    // 142 - Jeunesse - super grands
    // 144 - Passe-Partout - appli
    // 145 - Jeunesse
    // 149 - Avant de partir
    // 150 - Balados jeunesse
    // 152 - MAMMOUTH
    // 156 - Cette histoire nous mènera loin

    public const MENU_OPTIONS = [
        "documentary" => 106,
        "movie" => 110,
        "serie" => 137,
    ];

    public const API_URL = "https://beacon.playback.api.brightcove.com/telequebec/api/";
    public const ASSETS_URL = self::API_URL . "assets/";
    public const RECOMMENDATIONS_URL = self::API_URL . "menus/default/option/";

    // Types: web,Android,iOS,appletv,Firetv,Samsung,androidtv
    public const DEFAULT_PARAMETERS = [
        "device_layout" => "web",
        "device_type" => "web",
    ];

    public const DEFAULT_SEASON_NUMBER = 1; // To use when it is not a movies-type
    public const DEFAULT_SEASON_TITLE = "Saison 1"; // Follows the naming convention of Telequebec
    public const DEFAULT_EPISODE_NUMBER = 1; // ''

    public const TELEQUEBEC_SEARCH_URL = self::API_URL . "search/list_all";
    public const TELEQUEBEC_PARAMETERS_SEARCH = [
        "device_layout" => "web",
        "device_type" => "web",
        "limit" => 0,
        "asset_types" => "movies,series", // Types: movies, series, channels, seasons, episodes
        "q" => "",
    ];

    public const TELEQUEBEC_URL_SHOW_RECOMMENDATIONS = self::ASSETS_URL;

    public const TELEQUEBEC_PARAMETERS_SHOW_RECOMMENDATIONS = self::DEFAULT_PARAMETERS;

    public const TELEQUEBEC_URL_MOVIE_RECOMMENDATIONS =
        self::RECOMMENDATIONS_URL . self::MENU_OPTIONS["movie"];

    public const TELEQUEBEC_PARAMETERS_MOVIE_RECOMMENDATIONS = self::DEFAULT_PARAMETERS;

    public const TELEQUEBEC_URL_SERIE_RECOMMENDATIONS =
        self::RECOMMENDATIONS_URL . self::MENU_OPTIONS["serie"];

    public const TELEQUEBEC_PARAMETERS_SERIE_RECOMMENDATIONS = self::DEFAULT_PARAMETERS;

    public const TELEQUEBEC_URL_DOCUMENTARY_RECOMMENDATIONS =
        self::RECOMMENDATIONS_URL . self::MENU_OPTIONS["documentary"];

    public const TELEQUEBEC_PARAMETERS_DOCUMENTARY_RECOMMENDATIONS = self::DEFAULT_PARAMETERS;

    public const TELEQUEBEC_URL_SHOW_INFO = self::ASSETS_URL;

    public const TELEQUEBEC_PARAMETERS_SHOW_INFO = self::DEFAULT_PARAMETERS;

    public const TELEQUEBEC_URL_SEASON_INFO = self::ASSETS_URL;

    public const TELEQUEBEC_PARAMETERS_SEASON_INFO = self::DEFAULT_PARAMETERS;

    public const TELEQUEBEC_URL_SERIE_EPISODES_INFO = self::API_URL . "tvshow/";

    public const TELEQUEBEC_PARAMETERS_SERIE_EPISODES_INFO = [
        "device_layout" => "web",
        "device_type" => "web",
        "layout_id" => 1, // Default on web is 317 but it doesn't matter
        "limit" => 9223372036854775807,
    ];

    public const TELEQUEBEC_URL_EPISODE_INFO = self::ASSETS_URL;

    public const TELEQUEBEC_PARAMETERS_EPISODE_INFO = self::DEFAULT_PARAMETERS;

    public const TELEQUEBEC_URL_EPISODE_STREAM_INFO = self::ASSETS_URL;

    public const TELEQUEBEC_URL_EPISODE_VIDEO = "https://edge.api.brightcove.com/playback/v1/accounts/";

    public const TELEQUEBEC_HEADERS_EPISODE_VIDEO = [
        "Accept" => "pk=",
    ];

    // Stream URL
    // self::ASSETS_URL . "28634/streams/3515?cohort=988910&device_type=web&device_layout=web"

    public const TELEQUEBEC_URL_EPISODE_DOWNLOAD_INFO = self::API_URL . "";

    public const TELEQUEBEC_PARAMETERS_EPISODE_DOWNLOAD_INFO = [
        "appCode" => "toutv",
        "output" => "json",
        "tech" => "dash",
        "manifestVersion" => 2,
        "idMedia" => null,
    ];
}
