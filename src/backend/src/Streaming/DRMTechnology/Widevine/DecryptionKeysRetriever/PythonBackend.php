<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology\Widevine\DecryptionKeysRetriever;

use App\Streaming\DRMTechnology\Widevine\Classes\DecryptionKeysRetriever;
use App\Streaming\Helpers\PythonBackend\PythonBackendHelper;

/**
 * Using the Python API to retrieve the keys
 */
class PythonBackend extends DecryptionKeysRetriever
{
    public function getDecryptionKeys(
        string $pssh,
        string $licenseUrl,
        array $licenseHeaders,
    ): array {
        $response = PythonBackendHelper::get(
            "decryptionKeys",
            compact(["pssh", "licenseUrl", "licenseHeaders"]),
        );

        if ($response->error) {
            throw $response->error;
        }

        $decryptionKeys = $response->value;

        return $decryptionKeys;
    }
}
