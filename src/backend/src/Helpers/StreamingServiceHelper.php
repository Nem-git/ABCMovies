<?php

declare(strict_types=1);

namespace App\Helpers;

require_once __DIR__ . "/../../config/constants.php";

class StreamingServiceHelper
{
    public static function getStreamUrl(string $streamingServiceTag, string $showId, string $seasonId, string $episodeId, string $tech): string
    {
        return PHP_URL_BACKEND . join("/", [strtolower($streamingServiceTag), $showId, $seasonId, $episodeId, STREAMING_TECH_TO_FILENAME[$tech]]);
    }

    public static function getEpisodeDatabaseIdentifier(string $streamingServiceTag, string $showId, string $seasonId, string $episodeId): string
    {
        return join("/", [strtolower($streamingServiceTag), $showId, $seasonId, $episodeId]);
    }

    public static function getRecommendationMethodName(string $type): string
    {
        // if type is valid recommendation type
        if (in_array(strtolower($type), RECOMMENDATION_TYPES, true)) {
            return "execute".self::getPascalCaseWord($type)."Recommendations";
        }

        return "";
    }

    public static function getPascalCaseWord(string $word): string
    {
        return ucfirst(strtolower($word));
    }

}
