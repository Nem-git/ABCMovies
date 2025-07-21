<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology\Widevine\SegmentDecryptor;

use App\Streaming\DRMTechnology\Widevine\Classes\SegmentDecryptor;

/**
 * Using the Python API to decrypt the segment
 */
final class PythonBackend extends SegmentDecryptor
{
    #[\Override]
    public function getDecryptedSegment(
        $segmentContent,
        $initContent,
        $decryptionKeys = [],
        $shouldBeMerged = false,
    ): string {
        return "";
    }
}
