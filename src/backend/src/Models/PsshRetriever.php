<?php

declare(strict_types=1);

namespace App\Models;

use App\Helpers\RequestHelper;

abstract class PsshRetriever
{
    abstract public function getPssh(DownloadInfo $downloadInfo): DownloadInfo;
}
