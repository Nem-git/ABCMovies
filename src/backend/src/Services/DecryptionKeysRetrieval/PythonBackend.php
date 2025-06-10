<?php

declare(strict_types=1);

namespace App\Services\DecryptionKeysRetrieval;

use App\Models\DecryptionKeysRetrieval;
use App\Models\DownloadInfo;

/**
 * Using the Python API to retrieve the keys
 */
class PythonBackend extends DecryptionKeysRetrieval
{
    public function getDecryptionKeys(DownloadInfo $downloadInfo): DownloadInfo
    {
        $downloadInfo->decryptionKeys = $this->request->pythonBackend("decrypt", $downloadInfo);
        return $downloadInfo;
    }

}
