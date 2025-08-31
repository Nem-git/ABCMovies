<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology\Widevine\DecryptionKeysRetriever;

use App\Streaming\DRMTechnology\Widevine\Classes\DecryptionKeysRetriever;
use App\Streaming\Helpers\GoBackend\GoBackendHelper;

/**
 * Using the Go API to retrieve the keys
 */
final class GoBackend extends DecryptionKeysRetriever
{
    #[\Override]
    public function getDecryptionKeys(
        string $pssh,
        string $licenseUrl,
        array $licenseHeaders,
    ): array {
        $response = GoBackendHelper::get(
            "widevine/keys",
            [
            "pssh" => $pssh,
            "url" => $licenseUrl,
            "headers" => $licenseHeaders,
            ]
        );

        if ($response->error) {
            // TODO: Throw error
        }

        return $response->keys;
    }
}
