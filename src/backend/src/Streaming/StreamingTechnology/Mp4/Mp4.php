<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology\Mp4;

use App\Streaming\StreamingTechnology\StreamingTechnology;
use App\Streaming\Classes\Episode;
use App\Streaming\Classes\Season;
use App\Streaming\Classes\Show;

/**
 * Just a plain old MP4 file that gets "streamed" using bytes request in headers
 */
final class Mp4 extends StreamingTechnology
{
    public string $name = "mp4";
    public string $mimeType = "video/mp4";

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
