<?php

declare(strict_types=1);

namespace App\Services\PsshRetrieval;

use App\Models\PsshRetrieval;
use App\Models\DownloadInfo;

/**
 * Using the Python API to retrieve the keys
 */
class PythonBackend extends PsshRetrieval
{
    public function getPssh(DownloadInfo $downloadInfo): DownloadInfo
    {
        $downloadInfo->pssh = $this->request->pythonBackend("pssh", $downloadInfo);
        return $downloadInfo;
    }

}
