<?php

//region ENV

define(
    "REDIS_CONFIG",
    [
    "scheme" => "tcp",
    "host" => $_ENV["DB_HOST"],
    "port" => $_ENV["DB_PORT"],
    "protocol" => 3,
    "password" => $_ENV["DB_PW"],
    "database" => $_ENV["DB_ID"],
    ]
);

//endregion

define("DEFAULT_SEARCH_RESULTS_AMOUNT", 20);
define("DEFAULT_RECOMMENDATIONS_AMOUNT", 50);
define("DEFAULT_REDIS_TTL_TYPE", "EX"); // EX seconds, PX milliseconds, https://redis.io/docs/latest/commands/set/
define("DEFAULT_INIT_CONTENT_TTL", 60 * 60 * 24);

define(
    "RECOMMENDATION_TYPES",
    [
        "movies",
        "series",
        "documentaries",
    ]
);

define(
    "STREAMING_TECH_TO_FILENAME",
    [
        "dash" => "manifest.mpd",
        "hls" => "master.m3u8",
    ]
);

define(
    "WORD_TO_STREAMING_TECH",
    [
        "dash" => "dash",
        "manifest.mpd" => "dash",
        "hls" => "hls",
        "master.m3u8" => "hls",
    ]
);

define(
    "STREAMING_TECH_RANK",
    [
        "dash",
        "hls",
        "mp4",
        "smooth",
    ]
);

define(
    "HTTP_DEFAULT_HEADERS",
    [
    "User-Agent" => "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:109.0) Gecko/20100101 Firefox/109.0",
    "Accept" => "application/json",
    "Accept-Language" => "fr-CA,fr",
    "Sec-GPC" => "1",
    "Sec-Fetch-Dest" => "document",
    "Sec-Fetch-Mode" => "naviguate",
    "Sec-Fetch-Site" => "cross-site",
    "Priority" => "u=0, i",
    "Pragma" => "no-cache",
    "Cache-Control" => "no-cache"
    ]
);

define("TOUTV_URL_SEARCH", "https://services.radio-canada.ca/ott/catalog/v2/toutv/search");
define(
    "TOUTV_PARAMETERS_SEARCH",
    [
    "device" => "web",
    "pageSize" => 0,
    "term" => ""
    ]
);

define("TOUTV_URL_SHOW_RECOMMENDATIONS", "https://services.radio-canada.ca/ott/catalog/v2/toutv/show/");
define(
    "TOUTV_PARAMETERS_SHOW_RECOMMENDATIONS",
    [
    "device" => "web",
    "pageSize" => 0,
    ]
);

define("TOUTV_URL_MOVIES_RECOMMENDATIONS", "https://services.radio-canada.ca/ott/catalog/v2/toutv/category/film");
define(
    "TOUTV_PARAMETERS_MOVIES_RECOMMENDATIONS",
    [
    "device" => "web",
    "pageSize" => 0,
    ]
);

define("TOUTV_URL_SERIES_RECOMMENDATIONS", "https://services.radio-canada.ca/ott/catalog/v2/toutv/category/serie");
define(
    "TOUTV_PARAMETERS_SERIES_RECOMMENDATIONS",
    [
    "device" => "web",
    "pageSize" => 0,
    ]
);

define("TOUTV_URL_DOCUMENTARIES_RECOMMENDATIONS", "https://services.radio-canada.ca/ott/catalog/v2/toutv/category/documentaire");
define(
    "TOUTV_PARAMETERS_DOCUMENTARIES_RECOMMENDATIONS",
    [
    "device" => "web",
    "pageSize" => 0,
    ]
);

define("TOUTV_URL_SHOW_INFO", "https://services.radio-canada.ca/ott/catalog/v2/toutv/show/");
define(
    "TOUTV_PARAMETERS_SHOW_INFO",
    [
    "device" => "web"
    ]
);

define("TOUTV_URL_SEASON_INFO", "https://services.radio-canada.ca/ott/catalog/v2/toutv/show/");
define(
    "TOUTV_PARAMETERS_SEASON_INFO",
    [
    "device" => "web"
    ]
);

define("TOUTV_URL_EPISODE_INFO", "https://services.radio-canada.ca/ott/external/v1/toutv/showanalytics/");
define(
    "TOUTV_PARAMETERS_EPISODE_INFO",
    [
    "device" => "web"
    ]
);

define("TOUTV_URL_EPISODE_FILE_INFO", "https://services.radio-canada.ca/media/meta/v1/index.ashx");
define(
    "TOUTV_PARAMETERS_EPISODE_FILE_INFO",
    [
    "appCode" => "toutv",
    "output" => "jsonObject",
    "idMedia" => null
    ]
);

define("TOUTV_URL_EPISODE_DOWNLOAD_INFO", "https://services.radio-canada.ca/media/validation/v2/");
define(
    "TOUTV_PARAMETERS_EPISODE_DOWNLOAD_INFO",
    [
    "appCode" => "toutv",
    "output" => "json",
    "tech" => "dash",
    "manifestVersion" => 2,
    "idMedia" => null
    ]
);
define(
    "TOUTV_HEADERS_EPISODE_DOWNLOAD_INFO",
    [
    "Authorization" => null,
    "x-claims-token" => null
    ]
);
define(
    "TOUTV_HEADERS_EPISODE_DOWNLOAD_LICENSE_INFO",
    [
    "x-dt-auth-token" => null
    ]
);
