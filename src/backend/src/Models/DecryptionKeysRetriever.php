<?php

declare(strict_types=1);

namespace App\Models;

abstract class DecryptionKeysRetriever
{
    abstract public function getDecryptionKeys(DownloadInfo $downloadInfo): DownloadInfo;
}
