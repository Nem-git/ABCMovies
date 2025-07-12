<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology\Playready;

use App\Streaming\Classes\Episode;
use App\Streaming\DRMTechnology\DRMTechnology;

/**
 * Microsoft's DRM Technology
 */
class Playready extends DRMTechnology
{
    // https://git.gay/ready-dl/pyplayready

    public string $name = "playready";

    public function saveData(
        Episode $episode,
        string $episodeStreamingDrmTechnologyIdentifier,
    ): void {
    }

    public function getSegment(
        string $episodeStreamingDrmTechnologyIdentifier,
        string $initMediaIdentifier,
        string $reconstructedUrl,
        bool $isInit = false,
    ): string {
        return "";
    }
}
