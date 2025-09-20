<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology\Dash\ManifestModifier;

use App\Streaming\StreamingTechnology\Dash\Classes\ManifestModifier;
use App\Streaming\Helpers\GoBackend\GoBackendHelper;

/**
 * Using the Go API to modifiy the Dash Manifest
 */
final class GoBackend extends ManifestModifier
{
    #[\Override]
    public function getModifiedMpd(string $mpdUrl, string $mpdContent): string
    {
        $response = GoBackendHelper::get(
            "stream/dash/manifest",
            [
            "url" => $mpdUrl,
            "content" => $mpdContent,
            ]
        );

        if (isset($response->error)) {
            // TODO: Throw error
        }

        return $response->content;
    }
}
