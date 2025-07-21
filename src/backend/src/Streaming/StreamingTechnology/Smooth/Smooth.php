<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology\Smooth;

use Psr\Http\Message\ServerRequestInterface as Request;
use App\Streaming\StreamingTechnology\StreamingTechnology;
use App\Streaming\Classes\Episode;

/**
 * Microsoft's adaptative streaming technology
 */
final class Smooth extends StreamingTechnology
{
    public string $name = "smooth";
    public string $mimeType = "video/mp4"; // I don't know, can't find documentation :(

    #[\Override]
    public function getVideo(
        Request $request,
        Episode $episode,
        string $showId,
        string $seasonId,
        array $args = [],
    ): string
    {
        return "";
    }
}
