<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology\Widevine\PsshRetriever;

use App\Streaming\DRMTechnology\Widevine\Classes\PsshRetriever;
use App\Streaming\Classes\DownloadInfo;
use App\Streaming\Helpers\RequestHelper;

/**
 * Using the Python API to retrieve the keys
 */
class PythonBackend extends PsshRetriever
{
    public function getPssh(DownloadInfo $downloadInfo): DownloadInfo
    {
        $downloadInfo->pssh = RequestHelper::pythonBackend("pssh", $downloadInfo);
        return $downloadInfo;
    }

}
