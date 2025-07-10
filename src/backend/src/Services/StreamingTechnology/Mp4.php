<?php

declare(strict_types=1);

namespace App\Services\StreamingTechnology;

use Psr\Http\Message\ServerRequestInterface as Request;
use App\Models\StreamingTechnology;
use App\Services\StreamingService;

/**
 * Just a plain old MP4 file that gets "streamed" using bytes request in headers
 */
class Mp4 extends StreamingTechnology
{
    public string $name = "mp4";
    public string $mimeType = "video/mp4";

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
