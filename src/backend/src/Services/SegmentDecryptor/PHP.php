<?php

declare(strict_types=1);

namespace App\Services\SegmentDecryptor;

use App\Models\SegmentDecryptor;

/**
 * Using PHP to merge and decrypt the segment
 */
class Php extends SegmentDecryptor
{
    // TODO: Look into FFI for direct function calls the a shared library (need to recompile)
    public function getDecryptedSegment($initContent, $segmentContent, $decryptionKeys): string
    {

    }

}
