<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology\Dash\ManifestModifier;

use App\Streaming\StreamingTechnology\Dash\Classes\ManifestModifier;
use App\Streaming\Helpers\PythonBackend\PythonBackendHelper;

/**
 * Using the Python API to modifiy the Dash Manifest
 */
final class PythonBackend extends ManifestModifier
{
    #[\Override]
    public function getModifiedMpd(string $mpdUrl, string $mpdContent): string
    {
        $response = PythonBackendHelper::get(
            "manifest",
            compact(["mpdUrl", "mpdContent"]),
        );

        if ($response->error) {
            // TODO: Throw error
        }

        $modifiedManifestContent = $response->value;

        return $modifiedManifestContent;
    }
}
