<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology\Mp4;

use Psr\Http\Message\ServerRequestInterface as Request;
use App\Streaming\StreamingTechnology\StreamingTechnology;
use App\Streaming\Classes\Episode;

/**
 * Just a plain old MP4 file that gets "streamed" using bytes request in headers
 */
final class Mp4 extends StreamingTechnology
{
    public string $name = "mp4";
    public string $mimeType = "video/mp4";

    #[\Override]
    public function getVideo(
        Request $request,
        Episode $episode,
        string $showId,
        string $seasonId,
        array $args = [],
    ): string {
        return "";
    }
}
