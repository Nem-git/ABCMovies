<?php

declare(strict_types=1);

namespace App\Streaming\StreamingService\Helpers;

use App\Config\Constants;

final class StreamingServiceHelper
{
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
}
