<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology\Fairplay;

use App\Streaming\DRMTechnology\DRMTechnology;
use App\Streaming\Classes\Episode;

/**
 * Apple's DRM Technology
 */
final class Fairplay extends DRMTechnology
{
    // https://deepwiki.com/lqvp/apple-music-downloader

    public string $name = "fairplay";

    #[\Override]
    public function saveData(
        Episode $episode,
        string $episodeStreamingDrmTechnologyIdentifier,
    ): void {
    }

    #[\Override]
    public function getSegment(
        string $episodeStreamingDrmTechnologyIdentifier,
        string $initMediaIdentifier,
        string $reconstructedUrl,
        bool $isInit = false,
    ): string {
        return "";
    }
}
