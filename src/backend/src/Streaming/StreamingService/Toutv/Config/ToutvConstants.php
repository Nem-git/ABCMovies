<?php

declare(strict_types=1);

namespace App\Streaming\StreamingService\Toutv\Config;

final class ToutvConstants
{
    public const TOUTV_URL_SEARCH = "https://services.radio-canada.ca/ott/catalog/v2/toutv/search";

    public const TOUTV_PARAMETERS_SEARCH = [
        "device" => "web",
        "pageSize" => 0,
        "term" => "",
    ];

    public const TOUTV_URL_SHOW_RECOMMENDATIONS = "https://services.radio-canada.ca/ott/catalog/v2/toutv/show/";

    public const TOUTV_PARAMETERS_SHOW_RECOMMENDATIONS = [
        "device" => "web",
        "pageSize" => 0,
    ];

    public const TOUTV_URL_MOVIES_RECOMMENDATIONS = "https://services.radio-canada.ca/ott/catalog/v2/toutv/category/film";

    public const TOUTV_PARAMETERS_MOVIES_RECOMMENDATIONS = [
        "device" => "web",
        "pageSize" => 0,
    ];

    public const TOUTV_URL_SERIES_RECOMMENDATIONS = "https://services.radio-canada.ca/ott/catalog/v2/toutv/category/serie";

    public const TOUTV_PARAMETERS_SERIES_RECOMMENDATIONS = [
        "device" => "web",
        "pageSize" => 0,
    ];

    public const TOUTV_URL_DOCUMENTARIES_RECOMMENDATIONS = "https://services.radio-canada.ca/ott/catalog/v2/toutv/category/documentaire";

    public const TOUTV_PARAMETERS_DOCUMENTARIES_RECOMMENDATIONS = [
        "device" => "web",
        "pageSize" => 0,
    ];

    public const TOUTV_URL_SHOW_INFO = "https://services.radio-canada.ca/ott/catalog/v2/toutv/show/";

    public const TOUTV_PARAMETERS_SHOW_INFO = [
        "device" => "web",
    ];

    public const TOUTV_URL_SEASON_INFO = "https://services.radio-canada.ca/ott/catalog/v2/toutv/show/";

    public const TOUTV_PARAMETERS_SEASON_INFO = [
        "device" => "web",
    ];

    public const TOUTV_URL_EPISODE_INFO = "https://services.radio-canada.ca/ott/external/v2/toutv/mediaanalytics/";

    public const TOUTV_PARAMETERS_EPISODE_INFO = [
        "device" => "web",
    ];

    public const TOUTV_URL_EPISODE_FILE_INFO = "https://services.radio-canada.ca/media/meta/v1/index.ashx";

    public const TOUTV_PARAMETERS_EPISODE_FILE_INFO = [
        "appCode" => "toutv",
        "output" => "jsonObject",
        "idMedia" => null,
    ];

    public const TOUTV_URL_EPISODE_DOWNLOAD_INFO = "https://services.radio-canada.ca/media/validation/v2/";

    public const TOUTV_PARAMETERS_EPISODE_DOWNLOAD_INFO = [
        "appCode" => "toutv",
        "output" => "json",
        "tech" => "dash",
        "manifestVersion" => 2,
        "idMedia" => null,
    ];

    public const TOUTV_HEADERS_EPISODE_DOWNLOAD_INFO = [
        "Authorization" => null,
        "x-claims-token" => null,
    ];

    public const TOUTV_HEADERS_EPISODE_DOWNLOAD_LICENSE_INFO = [
        "x-dt-auth-token" => null,
    ];
}
