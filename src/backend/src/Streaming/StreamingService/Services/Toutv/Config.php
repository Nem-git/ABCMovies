<?php

declare(strict_types=1);

namespace App\Streaming\StreamingService\Services\Toutv;

use App\Streaming\StreamingService\Helpers\StreamingServiceHelper;

final class Config
{
    public const URL_SEARCH = "https://services.radio-canada.ca/ott/catalog/v2/toutv/search";

    public const PARAMETERS_SEARCH = [
        "device" => "web",
        "pageSize" => 0,
        "term" => "",
    ];

    public const SECOND_URL_SEARCH = "https://services.radio-canada.ca/ott/catalog/v1/toutv/search";

    public const SECOND_PARAMETERS_SEARCH = [
        "device" => "web",
        "pageSize" => 0,
        "term" => "",
    ];

    public const URL_RECOMMENDATION_TYPES = "https://services.radio-canada.ca/ott/catalog/v3/toutv/browse";

    public const PARAMETERS_RECOMMENDATION_TYPES = [
        "device" => "web",
    ];

    public const URL_SHOW_RECOMMENDATIONS = "https://services.radio-canada.ca/ott/catalog/v2/toutv/show/";

    public const PARAMETERS_SHOW_RECOMMENDATIONS = [
        "device" => "web",
        "pageSize" => 0,
    ];

    public const URL_MOVIES_RECOMMENDATIONS = "https://services.radio-canada.ca/ott/catalog/v2/toutv/category/film";

    public const PARAMETERS_MOVIES_RECOMMENDATIONS = [
        "device" => "web",
        "pageSize" => 0,
    ];

    public const URL_SERIES_RECOMMENDATIONS = "https://services.radio-canada.ca/ott/catalog/v2/toutv/category/serie";

    public const PARAMETERS_SERIES_RECOMMENDATIONS = [
        "device" => "web",
        "pageSize" => 0,
    ];

    public const URL_DOCUMENTARIES_RECOMMENDATIONS = "https://services.radio-canada.ca/ott/catalog/v2/toutv/category/documentaire";

    public const PARAMETERS_DOCUMENTARIES_RECOMMENDATIONS = [
        "device" => "web",
        "pageSize" => 0,
    ];

    public const URL_SHOW_INFO = "https://services.radio-canada.ca/ott/catalog/v2/toutv/show/";

    public const PARAMETERS_SHOW_INFO = [
        "device" => "web",
    ];

    public const URL_SEASON_INFO = "https://services.radio-canada.ca/ott/catalog/v2/toutv/show/";

    public const PARAMETERS_SEASON_INFO = [
        "device" => "web",
    ];

    public const URL_EPISODE_INFO = "https://services.radio-canada.ca/ott/external/v2/toutv/mediaanalytics/";

    public const PARAMETERS_EPISODE_INFO = [
        "device" => "web",
    ];

    public const URL_EPISODE_FILE_INFO = "https://services.radio-canada.ca/media/meta/v1/index.ashx";

    public const PARAMETERS_EPISODE_FILE_INFO = [
        "appCode" => "toutv",
        "output" => "jsonObject",
        "idMedia" => null,
    ];

    public const URL_EPISODE_DOWNLOAD_INFO = "https://services.radio-canada.ca/media/validation/v2/";

    public const PARAMETERS_EPISODE_DOWNLOAD_INFO = [
        "appCode" => "toutv",
        "output" => "json",
        "tech" => "dash",
        "manifestVersion" => 2,
        "idMedia" => null,
    ];

    public const HEADERS_EPISODE_DOWNLOAD_INFO = [
        "Authorization" => null,
        "x-claims-token" => null,
    ];

    public const HEADERS_EPISODE_DOWNLOAD_LICENSE_INFO = [
        "x-dt-auth-token" => null,
    ];

    // Login

    public const LOGIN_SETTINGS_RE = "/SETTINGS\s*=\s*({.*?});/";

    public const LOGIN_BASE_URL = "https://login.cbc.radio-canada.ca/bef1b538-1950-4283-9b27-b096cbc18070/B2C_1A_SSO_Login/";

    public const LOGIN_URL = self::LOGIN_BASE_URL . "oauth2/v2.0/authorize";
    public const LOGIN_CLIENT_ID = "ebe6e7b0-3cc3-463d-9389-083c7b24399c";
    public const LOGIN_REDIRECT_URL = "https://ici.tou.tv/auth-changed";

    public const LOGIN_RESPONSE_TYPE = "code"; //  id_token%20token
    public const LOGIN_RESPONSE_MODE = "fragment";
    public const LOGIN_CODE_CHALLENGE_METHOD = "S256";

    public const LOGIN_SELF_ASSERTED_URL =
        self::LOGIN_BASE_URL . "SelfAsserted"; // SelfAsserted
    public const LOGIN_SELF_ASSERTED_PARAMETERS = [
        "StateProperties" => "",
        "csrf_token" => "",
    ];
    public const LOGIN_SELF_ASSERTED_HEADERS = [
        "X-CSRF-TOKEN" => "",
        "Cookie" => "",
    ];
    public const LOGIN_SELF_ASSERTED_DATA = [
        "request_type" => "RESPONSE",
        "email" => "",
        "password" => "",
    ];

    public const LOGIN_CONFIRMED_URL =
        self::LOGIN_BASE_URL . "api/CombinedSigninAndSignup/confirmed";
    public const LOGIN_CONFIRMED_PARAMETERS = [
        "StateProperties" => "",
        "csrf_token" => "",
    ];
    public const LOGIN_CONFIRMED_HEADERS = [
        "Cookie" => "",
    ];

    public static function GET_LOGIN_NONCE(): string
    {
        return StreamingServiceHelper::generateUuid();
    }

    public static function GET_LOGIN_STATE(): string
    {
        return StreamingServiceHelper::base64Urlencode(
            StreamingServiceHelper::generateUuid() .
                "|" .
                json_encode(
                    [
                    "action" => "login",
                    "returnUrl" => "/",
                    ]
                ),
        );
    }

    public static function GET_CODE_CHALLENGE(): string
    {
        $codeVerifier = StreamingServiceHelper::generateCodeVerifier();
        return StreamingServiceHelper::generateCodeChallenge($codeVerifier);
    }

    public const LOGIN_OAUTH_PERMISSION_BASE_URL = "https://rcmnb2cprod.onmicrosoft.com/84593b65-0ef6-4a72-891c-d351ddd50aab/";
    public const LOGIN_SCOPE = [
        "openid",
        "offline_access",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL . "email",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL . "id.account.create",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL . "id.account.delete",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL . "id.account.info",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL . "id.account.modify",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL . "id.account.reset-password",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL .
        "id.account.send-confirmation-email",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL . "id.write",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL . "media-drmt",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL . "media-meta",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL . "media-validation",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL . "media-validation.read",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL . "metrik",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL . "norah.write",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL . "oidc4ropc",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL . "ott-profiling",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL . "ott-subscription",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL . "profile",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL . "subscriptions.validate",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL . "subscriptions.write",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL . "toutv",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL . "toutv-presentation",
        self::LOGIN_OAUTH_PERMISSION_BASE_URL . "toutv-profiling",
    ];

    public const LOGIN_PARAMETERS = [
        "client_id" => self::LOGIN_CLIENT_ID,
        "nonce" => "",
        "redirect_uri" => "",
        "scope" => "",
        "response_type" => self::LOGIN_RESPONSE_TYPE,
        "response_mode" => self::LOGIN_RESPONSE_MODE,
        "response_mode_value" => self::LOGIN_RESPONSE_MODE,
        "state" => "",
        "state_value" => "",
        "code_challenge" => "",
        "code_challenge_method" => self::LOGIN_CODE_CHALLENGE_METHOD,
    ];
}
