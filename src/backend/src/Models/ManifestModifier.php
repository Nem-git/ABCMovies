<?php

declare(strict_types=1);

namespace App\Models;

use App\Helpers\RequestHelper;

abstract class ManifestModifier
{
    protected RequestHelper $request;

    public function __construct(RequestHelper $requestHelper)
    {
        $this->request = $requestHelper;
    }

    abstract public function getModifiedMpd(DownloadInfo $downloadInfo): string;
}
