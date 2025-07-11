<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology\Widevine\DecryptionKeysRetriever;

use App\Streaming\DRMTechnology\Widevine\Classes\DecryptionKeysRetriever;
use App\Streaming\Classes\DownloadInfo;
use App\Streaming\Helpers\RequestHelper;

/**
 * Using the Python API to retrieve the keys
 */
class PythonBackend extends DecryptionKeysRetriever
{
    public function getDecryptionKeys(DownloadInfo $downloadInfo): DownloadInfo
    {
        $downloadInfo->decryptionKeys = RequestHelper::pythonBackend(
            "decrypt",
            $downloadInfo,
        );
        return $downloadInfo;
    }
}
