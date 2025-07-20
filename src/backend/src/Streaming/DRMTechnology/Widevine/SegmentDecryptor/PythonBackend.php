<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology\Widevine\SegmentDecryptor;

use App\Streaming\DRMTechnology\Widevine\Classes\SegmentDecryptor;

/**
 * Using the Python API to decrypt the segment
 */
class PythonBackend extends SegmentDecryptor
{
    public function getDecryptedSegment(
        $initContent,
        $segmentContent,
        $decryptionKeys,
        $shouldBeMerged = false,
    ): string {
        return "";
    }
}
