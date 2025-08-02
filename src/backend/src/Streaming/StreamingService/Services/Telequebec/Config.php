<?php

declare(strict_types=1);

namespace App\Streaming\StreamingService\Services\Telequebec;

final class Config
{
    public const DEFAULT_SEASON_NUMBER = 1; // To use when it is not a movies-type
    public const DEFAULT_SEASON_TITLE = "Saison 1"; // Follows the naming convention of Telequebec
    public const DEFAULT_EPISODE_NUMBER = 1; // ''

    // https://beacon.brightcove.com/telequebec/api/search/list_all == https://beacon.playback.api.brightcove.com/telequebec/api/search/list_all
    public const TELEQUEBEC_SEARCH_URL = "https://beacon.playback.api.brightcove.com/telequebec/api/search/list_all";
    public const TELEQUEBEC_PARAMETERS_SEARCH = [
        "device_layout" => "web",
        "device_type" => "web",
        "limit" => 0,
        "asset_types" => "movies,series", // Types: movies, series, channels, seasons, episodes
        "q" => "",
    ];

    public const TELEQUEBEC_URL_SHOW_RECOMMENDATIONS = "https://beacon.playback.api.brightcove.com/telequebec/api/assets/";

    public const TELEQUEBEC_PARAMETERS_SHOW_RECOMMENDATIONS = [
        "device_layout" => "web",
        "device_type" => "web",
    ];

    public const TELEQUEBEC_URL_MOVIES_RECOMMENDATIONS = "https://services.radio-canada.ca/ott/catalog/v2/toutv/category/film";

    public const TELEQUEBEC_PARAMETERS_MOVIES_RECOMMENDATIONS = [
        "device" => "web",
        "pageSize" => 0,
    ];

    public const TELEQUEBEC_URL_SERIES_RECOMMENDATIONS = "https://services.radio-canada.ca/ott/catalog/v2/toutv/category/serie";

    public const TELEQUEBEC_PARAMETERS_SERIES_RECOMMENDATIONS = [
        "device" => "web",
        "pageSize" => 0,
    ];

    public const TELEQUEBEC_URL_DOCUMENTARIES_RECOMMENDATIONS = "https://services.radio-canada.ca/ott/catalog/v2/toutv/category/documentaire";

    public const TELEQUEBEC_PARAMETERS_DOCUMENTARIES_RECOMMENDATIONS = [
        "device" => "web",
        "pageSize" => 0,
    ];

    public const TELEQUEBEC_URL_SHOW_INFO = "https://beacon.playback.api.brightcove.com/telequebec/api/assets/";

    public const TELEQUEBEC_PARAMETERS_SHOW_INFO = [
        "device_layout" => "web",
        "device_type" => "web",
    ];

    public const TELEQUEBEC_URL_SEASON_INFO = "https://beacon.playback.api.brightcove.com/telequebec/api/assets/";

    public const TELEQUEBEC_PARAMETERS_SEASON_INFO = [
        "device_layout" => "web",
        "device_type" => "web",
    ];

    public const TELEQUEBEC_URL_SEASON_EPISODES_INFO = "https://beacon.playback.api.brightcove.com/telequebec/api/tvshow/";

    public const TELEQUEBEC_PARAMETERS_SEASON_EPISODES_INFO = [
        "device_layout" => "web",
        "device_type" => "web",
        "layout_id" => 1, // Default on web is 317 but it doesn't matter
        "limit" => 9223372036854775807,
    ];

    public const TELEQUEBEC_URL_EPISODE_INFO = "https://services.radio-canada.ca/ott/external/v2/toutv/mediaanalytics/";

    public const TELEQUEBEC_PARAMETERS_EPISODE_INFO = [
        "device" => "web",
    ];

    public const TELEQUEBEC_URL_EPISODE_FILE_INFO = "https://services.radio-canada.ca/media/meta/v1/index.ashx";

    public const TELEQUEBEC_PARAMETERS_EPISODE_FILE_INFO = [
        "appCode" => "toutv",
        "output" => "jsonObject",
        "idMedia" => null,
    ];

    public const TELEQUEBEC_URL_EPISODE_DOWNLOAD_INFO = "https://services.radio-canada.ca/media/validation/v2/";

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
