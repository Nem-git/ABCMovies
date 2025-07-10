<?php

declare(strict_types=1);

namespace App\Models;

use Psr\Http\Message\ServerRequestInterface as Request;
use App\Services\StreamingService;

abstract class StreamingTechnology
{
    public string $name;
    public string $mimeType;

    abstract public function getVideo(StreamingService $streamingService, Request $request, string $showId, string $seasonId, string $episodeId, array $args = []): string;

}
