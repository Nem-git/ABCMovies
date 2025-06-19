<?php

declare(strict_types=1);

namespace App\Models;

abstract class ManifestModifier
{
    abstract public function getModifiedMpd(DownloadInfo $downloadInfo): string;
}
