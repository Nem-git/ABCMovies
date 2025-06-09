<?php

declare(strict_types=1);

namespace App\Models;

use App\Helpers\RequestHelper;

abstract class PsshRetrieval
{
    protected RequestHelper $request;

    function __construct(RequestHelper $requestHelper)
    {
        $this->request = $requestHelper;
    }

    abstract public function getPssh(DownloadInfo $downloadInfo): DownloadInfo;
}