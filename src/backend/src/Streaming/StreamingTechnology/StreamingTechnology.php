<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology;

use App\Streaming\DRMTechnology\DRMTechnology;
use Psr\Http\Message\ServerRequestInterface as Request;
use App\Streaming\StreamingService\StreamingService;

abstract class StreamingTechnology
{
    public string $name;
    public string $mimeType;
    public DRMTechnology $drmTechnology;

    abstract public function getVideo(
        StreamingService $streamingService,
        Request $request,
        string $showId,
        string $seasonId,
        string $episodeId,
        array $args = [],
    ): string;
}
