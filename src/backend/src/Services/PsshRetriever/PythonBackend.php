<?php

declare(strict_types=1);

namespace App\Services\PsshRetriever;

use App\Helpers\RequestHelper;
use App\Models\DownloadInfo;
use App\Models\PsshRetriever;

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
