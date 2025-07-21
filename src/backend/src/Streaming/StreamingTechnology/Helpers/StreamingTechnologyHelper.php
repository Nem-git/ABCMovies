<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology\Helpers;

use App\Streaming\Helpers\RequestHelper;

final class StreamingTechnologyHelper
{
    public static function reconstructUrlFromArray(
        string $scheme = "https",
        array $urlPath = [],
        array $queryParams = [],
    ): string {
        $newUrl = $scheme . "://";

        $joinedUrlPath = join("/", $urlPath);

        $newUrl .= $joinedUrlPath;

        // Add the query params to the last part of the URL
        $newUrl .= RequestHelper::format_parameters($queryParams);

        return $newUrl;
    }

    public static function getEpisodeStreamingDRMTechnologyIdentifier(
        string $streamingServiceTag,
        string $showId,
        string $seasonId,
        string $episodeId,
        string $streamingTechnologyName,
        string $drmTechnologyName,
    ): string {
        return strtolower(
            join(
                "-",
                [
                $streamingServiceTag,
                $showId,
                $seasonId,
                $episodeId,
                $streamingTechnologyName,
                $drmTechnologyName,
                ]
            ),
        );
    }
}
