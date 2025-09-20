<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology\Widevine\PsshRetriever;

use App\Streaming\DRMTechnology\Widevine\Classes\PsshRetriever;
use App\Streaming\Helpers\GoBackend\GoBackendHelper;

/**
 * Using the Go API to retrieve the keys
 */
final class GoBackend extends PsshRetriever
{
    public function getPssh(
        string $mpdUrl,
        array $mpdHeaders,
        array $segmentHeaders,
    ): string {
        $response = GoBackendHelper::get(
            "drm/widevine/pssh",
            [
            "url" => $mpdUrl,
            "headers" => $mpdHeaders,
            "segheaders" => $segmentHeaders,
            ]
        );

        if (isset($response->error)) {
            // TODO: Throw error
            var_dump($response);
        }

        return $response->pssh;
    }
}
