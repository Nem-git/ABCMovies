<?php

declare(strict_types=1);

namespace App\Services\SegmentDecryptor;

use App\Models\SegmentDecryptor;

/**
 * Using the Python API to decrypt the segment
 */
class PythonBackend extends SegmentDecryptor
{
    public function getDecryptedSegment($initContent, $segmentContent, $decryptionKeys): string
    {

    }

}
