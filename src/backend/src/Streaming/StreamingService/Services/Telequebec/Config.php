<?php

declare(strict_types=1);

namespace App\Streaming\StreamingService\Services\Telequebec;

final class Config
{
    public const API_URL = "https://beacon.playback.api.brightcove.com/telequebec/api/";
    public const ASSETS_URL = self::API_URL . "assets/";

    // Types: web,Android,iOS,appletv,Firetv,Samsung,androidtv
    public const DEFAULT_PARAMETERS = [
        "device_layout" => "web",
        "device_type" => "web",
    ];

    public const DEFAULT_SEASON_NUMBER = 1; // To use when it is not a movies-type
    public const DEFAULT_SEASON_TITLE = "Saison 1"; // Follows the naming convention of Telequebec
    public const DEFAULT_EPISODE_NUMBER = 1; // ''

    // https://beacon.brightcove.com/telequebec/api/search/list_all == https://beacon.playback.api.brightcove.com/telequebec/api/search/list_all
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

    public const TELEQUEBEC_URL_MOVIES_RECOMMENDATIONS = self::API_URL . "";

    public const TELEQUEBEC_PARAMETERS_MOVIES_RECOMMENDATIONS = self::DEFAULT_PARAMETERS;

    public const TELEQUEBEC_URL_SERIES_RECOMMENDATIONS = "https://services.radio-canada.ca/ott/catalog/v2/toutv/category/serie";

    public const TELEQUEBEC_PARAMETERS_SERIES_RECOMMENDATIONS = self::DEFAULT_PARAMETERS;

    public const TELEQUEBEC_URL_DOCUMENTARIES_RECOMMENDATIONS = "https://services.radio-canada.ca/ott/catalog/v2/toutv/category/documentaire";

    public const TELEQUEBEC_PARAMETERS_DOCUMENTARIES_RECOMMENDATIONS = self::DEFAULT_PARAMETERS;

    public const TELEQUEBEC_URL_SHOW_INFO = self::ASSETS_URL;

    public const TELEQUEBEC_PARAMETERS_SHOW_INFO = self::DEFAULT_PARAMETERS;

    public const TELEQUEBEC_URL_SEASON_INFO = self::ASSETS_URL;

    public const TELEQUEBEC_PARAMETERS_SEASON_INFO = self::DEFAULT_PARAMETERS;

    public const TELEQUEBEC_URL_SEASON_EPISODES_INFO =
        self::API_URL . "tvshow/";

    public const TELEQUEBEC_PARAMETERS_SEASON_EPISODES_INFO = [
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

    public const TELEQUEBEC_HEADERS_EPISODE_DOWNLOAD_INFO = [
        "Authorization" => null,
        "x-claims-token" => null,
    ];

    public const TELEQUEBEC_HEADERS_EPISODE_DOWNLOAD_LICENSE_INFO = [
        "x-dt-auth-token" => null,
    ];
}
