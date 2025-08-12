<?php

declare(strict_types=1);

namespace App\Streaming\StreamingService\Helpers;

use App\Config\Constants;

final class StreamingServiceHelper
{
    private const PKCE_CHARSET = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~";

    public static function getStreamUrl(
        string $streamingServiceTag,
        string $showId,
        int $seasonNumber,
        int $episodeNumber,
        string $tech,
    ): string {
        return $_ENV["PHP_BACKEND_URL"] .
            join(
                "/",
                [
                strtolower($streamingServiceTag),
                $showId,
                (string) $seasonNumber,
                (string) $episodeNumber,
                Constants::STREAMING_TECH_TO_FILENAME[$tech],
                ]
            );
    }

    public static function getEpisodeDatabaseIdentifier(
        string $streamingServiceTag,
        string $showId,
        int $seasonNumber,
        int $episodeNumber,
    ): string {
        return join(
            "/",
            [
            strtolower($streamingServiceTag),
            $showId,
            $seasonNumber,
            $episodeNumber,
            ]
        );
    }

    public static function getRecommendationMethodName(string $type): string
    {
        // if type is valid recommendation type
        if (in_array(strtolower($type), Constants::RECOMMENDATION_TYPES, true)
        ) {
            return "execute" .
                self::getPascalCaseWord($type) .
                "Recommendations";
        }

        return "";
    }

    public static function getPascalCaseWord(string $word): string
    {
        return ucfirst(strtolower($word));
    }

    public static function base64Urlencode(string $str): string
    {
        return rtrim(strtr(base64_encode($str), "+/", "-_"), "=");
    }

    public static function generateCodeVerifier(int $length = 32): string
    {
        $code = "";

        for ($i = 0; $i < $length; $i++) {
            $code .=
                self::PKCE_CHARSET[
                    random_int(0, strlen(self::PKCE_CHARSET) - 1)
                ];
        }

        return $code;
    }

    public static function generateCodeChallenge(string $codeVerifier): string
    {
        $hash = hash("sha256", $codeVerifier, true);
        return sodium_bin2base64(
            $hash,
            SODIUM_BASE64_VARIANT_URLSAFE_NO_PADDING,
        );
    }

    public static function generateUuid(): string
    {
        return trim(file_get_contents("/proc/sys/kernel/random/uuid"));
    }
}
