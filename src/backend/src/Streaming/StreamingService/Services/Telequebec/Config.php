<?php

declare(strict_types=1);

namespace App\Streaming\StreamingService\Services\Telequebec;

final class Config
{
    public const API_URL = "https://beacon.playback.api.brightcove.com/telequebec/api/";
    public const ASSETS_URL = self::API_URL . "assets/";

    // Brightsight DHCP UUID
    // Don't know if it is different per device
    public const DUID = "2a0e9037-76fc-4d51-858f-7f7127afec44";

    // Retrieved from config.json
    public const ACCOUNT_ID = "6101674910001";
    public const POLICY_KEY = "BCpkADawqM2A4ZomkTDWD1LdUfsOmlQ_3Xi0TnQ58f8fhbLfO-_gdBd1jjmWSlhia8NfnpwVmoEF2-sHbJe8DuwnEUh7QUobKZptq-5pgp3eObByOvRsYRyor9RqMcqPMcV-Pj-XTuZNiIm3agaV3huSRu8by_g945jTng";

    // Episode info:
    // 3515: Stream ID retrieved from
    // https://beacon.playback.api.brightcove.com/telequebec/api/assets/28634/streams/3515?device_type=web&device_layout=web

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

    public const TELEQUEBEC_PARAMETERS_SHOW_RECOMMENDATIONS = [
        "device_layout" => "web",
        "device_type" => "web",
    ];

    public const TELEQUEBEC_URL_MOVIES_RECOMMENDATIONS = self::API_URL . "";

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

    public const TELEQUEBEC_URL_SHOW_INFO = self::ASSETS_URL;

    public const TELEQUEBEC_PARAMETERS_SHOW_INFO = [
        "device_layout" => "web",
        "device_type" => "web",
    ];

    public const TELEQUEBEC_URL_SEASON_INFO = self::ASSETS_URL;

    public const TELEQUEBEC_PARAMETERS_SEASON_INFO = [
        "device_layout" => "web",
        "device_type" => "web",
    ];

    public const TELEQUEBEC_URL_SEASON_EPISODES_INFO =
        self::API_URL . "tvshow/";

    public const TELEQUEBEC_PARAMETERS_SEASON_EPISODES_INFO = [
        "device_layout" => "web",
        "device_type" => "web",
        "layout_id" => 1, // Default on web is 317 but it doesn't matter
        "limit" => 9223372036854775807,
    ];

    public const TELEQUEBEC_URL_EPISODE_INFO = self::ASSETS_URL;

    public const TELEQUEBEC_PARAMETERS_EPISODE_INFO = [
        "device" => "web",
    ];

    public const TELEQUEBEC_HEADERS_EPISODE_INFO = [
        "Accept" => "pk=" . self::POLICY_KEY,
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

    public const TELEQUEBEC_URL_ANONYMOUS_LOGIN =
        self::API_URL . "account/anonymous_login";

    public const TELEQUEBEC_PARAMETERS_ANONYMOUS_LOGIN = [
        "device_type" => "web",
        "duid" => self::DUID,
    ];
}
