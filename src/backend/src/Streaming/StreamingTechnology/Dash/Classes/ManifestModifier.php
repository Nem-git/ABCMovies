<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology\Dash\Classes;

abstract class ManifestModifier
{
    abstract public function getModifiedMpd(
        string $mpdUrl,
        string $mpdContent,
    ): string;
}
