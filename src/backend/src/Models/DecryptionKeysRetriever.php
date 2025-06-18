<?php

declare(strict_types=1);

namespace App\Models;

use App\Helpers\RequestHelper;

abstract class DecryptionKeysRetriever
{
    abstract public function getDecryptionKeys(DownloadInfo $downloadInfo): DownloadInfo;
}
