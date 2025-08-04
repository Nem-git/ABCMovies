<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology\Hls;

use Psr\Http\Message\ServerRequestInterface as Request;
use App\Streaming\StreamingTechnology\StreamingTechnology;
use App\Streaming\Classes\Episode;
use App\Streaming\Classes\Season;
use App\Streaming\Classes\Show;

/**
 * Apple's adaptative streaming technology
 */
final class Hls extends StreamingTechnology
{
    public string $name = "hls";
    public string $mimeType = "application/vnd.apple.mpegurl";

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

    // private function getMaster()
    // {
    // }

    // private function getPlaylist()
    // {
    // }

    // private function getFragment()
    // {
    // }
}
