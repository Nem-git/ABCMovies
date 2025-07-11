<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology\Smooth;

use Psr\Http\Message\ServerRequestInterface as Request;
use App\Streaming\StreamingTechnology\StreamingTechnology;
use App\Streaming\StreamingService\StreamingService;

/**
 * Microsoft's adaptative streaming technology
 */
class Smooth extends StreamingTechnology
{
    public string $name = "smooth";
    public string $mimeType = "video/mp4"; // I don't know, can't find documentation :(

    public function getVideo(StreamingService $streamingService, Request $request, string $showId, string $seasonId, string $episodeId, array $args = []): string
    {
        return "";
    }
}
