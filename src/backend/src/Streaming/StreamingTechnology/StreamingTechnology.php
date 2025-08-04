<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology;

use App\Streaming\DRMTechnology\DRMTechnology;
use App\Streaming\Classes\Episode;
use App\Streaming\Classes\Season;
use App\Streaming\Classes\Show;

abstract class StreamingTechnology
{
    public string $name;
    public string $mimeType;
    public DRMTechnology $drmTechnology;

    abstract public function getVideo(
        Show $show,
        Season $season,
        Episode $episode,
        array $queryParams = [],
        array $args = [],
    ): string;
}
