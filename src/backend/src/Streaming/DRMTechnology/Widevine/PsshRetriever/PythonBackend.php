<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology\Widevine\PsshRetriever;

use App\Streaming\DRMTechnology\Widevine\Classes\PsshRetriever;
use App\Streaming\Helpers\PythonBackend\PythonBackendHelper;

/**
 * Using the Python API to retrieve the keys
 */
final class PythonBackend extends PsshRetriever
{
    public function getPssh(
        string $mpdUrl,
        array $mpdHeaders,
        array $segmentHeaders,
    ): string {
        $response = PythonBackendHelper::get(
            "pssh",
            [
                "mpdUrl" => $mpdUrl,
                "mpdHeaders" => $mpdHeaders,
                "segmentHeaders" => $segmentHeaders
            ],
        );

        if ($response->error) {
            throw $response->error;
        }

        $pssh = $response->value;

        return $pssh;
    }
}
