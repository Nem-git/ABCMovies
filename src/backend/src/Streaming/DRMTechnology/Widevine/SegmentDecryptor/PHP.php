<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology\Widevine\SegmentDecryptor;

use App\Streaming\DRMTechnology\Widevine\Classes\SegmentDecryptor;

/**
 * Using PHP to merge and decrypt the segment
 */
class Php extends SegmentDecryptor
{
    // TODO: Look into FFI for direct function calls the a shared library (need to recompile)
    public function getDecryptedSegment(
        $initContent,
        $segmentContent,
        $decryptionKeys,
        $shouldBeMerged = false,
    ): string {
        return "";
    }
}
