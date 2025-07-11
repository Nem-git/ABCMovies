<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology\Widevine\Classes;

abstract class PsshRetriever
{
    abstract public function getPssh(
        string $mpdUrl,
        array $mpdHeaders,
        array $segmentHeaders,
    ): string;
}
