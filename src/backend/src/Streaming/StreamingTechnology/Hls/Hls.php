<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology\Hls;

use Psr\Http\Message\ServerRequestInterface as Request;
use App\Streaming\StreamingTechnology\StreamingTechnology;
use App\Streaming\Classes\Episode;

/**
 * Apple's adaptative streaming technology
 */
final class Hls extends StreamingTechnology
{
    public string $name = "hls";
    public string $mimeType = "application/vnd.apple.mpegurl";

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
