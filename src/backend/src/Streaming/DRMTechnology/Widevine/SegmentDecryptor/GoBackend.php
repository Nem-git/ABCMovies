<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology\Widevine\SegmentDecryptor;

use App\Streaming\DRMTechnology\Widevine\Classes\SegmentDecryptor;
use App\Streaming\Helpers\BentoHelper;
use App\Streaming\Helpers\GoBackend\GoBackendHelper;

/**
 * Using the Go backend to decrypt segments
 */
final class GoBackend extends SegmentDecryptor
{
    #[\Override]
    public function getDecryptedSegment(
        $segmentContent,
        $initContent = "",
        $decryptionKeys = [],
        $shouldBeMerged = false,
    ): string {
        $response = GoBackendHelper::get(
            "drm/widevine/remove",
            [
            "init" => base64_encode($initContent),
            "segment" => base64_encode($segmentContent),
            "keys" => $decryptionKeys,
            ]
        );

        if (isset($response->error)) {
            // TODO: Throw error
            var_dump($response);
        }

        return base64_decode($response->segment, true);
    }
}
