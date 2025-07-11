<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology\Hls;

use Psr\Http\Message\ServerRequestInterface as Request;
use App\Streaming\StreamingTechnology\StreamingTechnology;
use App\Streaming\StreamingService\StreamingService;

/**
 * Apple's adaptative streaming technology
 */
class Hls extends StreamingTechnology
{
    public string $name = "hls";
    public string $mimeType = "application/vnd.apple.mpegurl";

    public function getVideo(StreamingService $streamingService, Request $request, string $showId, string $seasonId, string $episodeId, array $args = []): string
    {
        return "";
    }

    private function getMaster()
    {

    }


    private function getPlaylist()
    {
    }

    private function getFragment()
    {

    }
}
