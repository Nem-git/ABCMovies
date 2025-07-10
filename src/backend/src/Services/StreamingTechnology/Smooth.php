<?php

declare(strict_types=1);

namespace App\Services\StreamingTechnology;

use Psr\Http\Message\ServerRequestInterface as Request;
use App\Models\StreamingTechnology;
use App\Services\StreamingService;

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
