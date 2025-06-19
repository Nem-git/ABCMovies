<?php

define("PYTHON_URL_BACKEND", "localhost:8000/");
define('PHP_URL_BACKEND', "http://localhost/api/");
define("TEMP_DIR", "/tmp/");

define("DEFAULT_SEARCH_RESULTS_AMOUNT", 20);

define(
    "REDIS_CONFIG",
    [
    "scheme" => "tcp",
    "host" => "localhost",
    "port" => 6379,
    "protocol" => 3,
    "password" => "",
    "database" => 0
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
