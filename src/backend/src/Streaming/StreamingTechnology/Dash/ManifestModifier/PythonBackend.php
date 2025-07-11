<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology\Dash\ManifestModifier;

use App\Streaming\StreamingTechnology\Dash\Classes\ManifestModifier;
use App\Streaming\Classes\DownloadInfo;
use App\Streaming\Helpers\RequestHelper;

/**
 * Using the Python API to modifiy the Dash Manifest
 */
class PythonBackend extends ManifestModifier
{
    public function getModifiedMpd(DownloadInfo $downloadInfo): string
    {
        return RequestHelper::pythonBackend("mpd", $downloadInfo);
    }
}
