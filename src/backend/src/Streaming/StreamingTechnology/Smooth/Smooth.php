<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology\Smooth;

use App\Streaming\StreamingTechnology\StreamingTechnology;
use App\Streaming\Classes\Episode;
use App\Streaming\Classes\Season;
use App\Streaming\Classes\Show;

/**
 * Microsoft's adaptative streaming technology
 */
final class Smooth extends StreamingTechnology
{
    public string $name = "smooth";
    public string $mimeType = "video/mp4"; // I don't know, can't find documentation :(

    #[\Override]
    public function getVideo(
        Show $show,
        Season $season,
        Episode $episode,
        array $queryParams = [],
        array $args = [],
    ): string {
        return "";
    }
}
