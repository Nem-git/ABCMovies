<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology\Widevine\Classes;

use App\Streaming\Classes\DownloadInfo;

abstract class DecryptionKeysRetriever
{
    abstract public function getDecryptionKeys(
        DownloadInfo $downloadInfo,
    ): DownloadInfo;
}
