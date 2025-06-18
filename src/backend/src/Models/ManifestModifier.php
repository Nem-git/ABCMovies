<?php

declare(strict_types=1);

namespace App\Models;

use App\Helpers\RequestHelper;

abstract class ManifestModifier
{
    abstract public function getModifiedMpd(DownloadInfo $downloadInfo): string;
}
