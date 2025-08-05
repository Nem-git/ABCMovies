<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology\Helpers;

use App\Streaming\Helpers\RequestHelper;
use App\Streaming\Classes\Episode;
use App\Streaming\Classes\Season;
use App\Streaming\Classes\Show;

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
        Show $show,
        Season $season,
        Episode $episode,
    ): string {
        $id = [
            $episode->provider,
            $show->id,
            $season->number,
            $episode->number,
            $episode->streamingTechnology->name,
        ];

        // Only add the drm IF the content is DRM protected
        if (isset($episode->streamingTechnology->drmTechnology)) {
            $id[] = $episode->streamingTechnology->drmTechnology->name;
        }

        return strtolower(join("-", $id));
    }
}
