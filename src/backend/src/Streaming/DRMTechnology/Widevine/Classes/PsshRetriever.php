<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology\Widevine\Classes;

use App\Streaming\Classes\DownloadInfo;

abstract class PsshRetriever
{
    abstract public function getPssh(DownloadInfo $downloadInfo): DownloadInfo;
}
