<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology;

use App\Streaming\DRMTechnology\DRMTechnology;
use Psr\Http\Message\ServerRequestInterface as Request;
use App\Streaming\Classes\Episode;

abstract class StreamingTechnology
{
    public string $name;
    public string $mimeType;
    public DRMTechnology $drmTechnology;

    abstract public function getVideo(
        Request $request,
        Episode $episode,
        string $showId,
        string $seasonId,
        array $args = [],
    ): string;
}
