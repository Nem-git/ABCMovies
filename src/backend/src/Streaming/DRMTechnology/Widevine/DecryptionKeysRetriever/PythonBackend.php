<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology\Widevine\DecryptionKeysRetriever;

use App\Streaming\DRMTechnology\Widevine\Classes\DecryptionKeysRetriever;
use App\Streaming\Helpers\PythonBackend\PythonBackendHelper;

/**
 * Using the Python API to retrieve the keys
 */
final class PythonBackend extends DecryptionKeysRetriever
{
    #[\Override]
    public function getDecryptionKeys(
        string $pssh,
        string $licenseUrl,
        array $licenseHeaders,
    ): array {
        $response = PythonBackendHelper::get(
            "decryptionKeys",
            [
            "pssh" => $pssh,
            "licenseUrl" => $licenseUrl,
            "licenseHeaders" => $licenseHeaders,
            ]
        );

        if ($response->error) {
            // TODO: Throw error
        }

        $decryptionKeys = (array) $response->value;

        return $decryptionKeys;
    }
}
