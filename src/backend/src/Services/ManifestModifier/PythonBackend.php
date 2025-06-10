<?php

declare(strict_types=1);

namespace App\Services\ManifestModifier;

use App\Models\ManifestModifier;
use App\Models\DownloadInfo;

/**
 * Using the Python API to modifiy the Dash Manifest
 */
class PythonBackend extends ManifestModifier
{
    public function getModifiedMpd(DownloadInfo $downloadInfo): string
    {
        return $this->request->pythonBackend("mpd", $downloadInfo);
    }
}
