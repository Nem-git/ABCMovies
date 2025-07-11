<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology\Dash\Classes;

use App\Streaming\Classes\DownloadInfo;

abstract class ManifestModifier
{
    abstract public function getModifiedMpd(DownloadInfo $downloadInfo): string;
}
