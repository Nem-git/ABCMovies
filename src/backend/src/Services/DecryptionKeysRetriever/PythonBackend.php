<?php

declare(strict_types=1);

namespace App\Services\DecryptionKeysRetriever;

use App\Models\DecryptionKeysRetriever;
use App\Models\DownloadInfo;
use App\Helpers\RequestHelper;

/**
 * Using the Python API to retrieve the keys
 */
class PythonBackend extends DecryptionKeysRetriever
{
    public function getDecryptionKeys(DownloadInfo $downloadInfo): DownloadInfo
    {
        $downloadInfo->decryptionKeys = RequestHelper::pythonBackend("decrypt", $downloadInfo);
        return $downloadInfo;
    }
}
