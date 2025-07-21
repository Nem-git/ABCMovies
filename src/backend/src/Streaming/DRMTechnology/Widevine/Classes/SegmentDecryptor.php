<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology\Widevine\Classes;

abstract class SegmentDecryptor
{
    abstract public function getDecryptedSegment(
        string $segmentContent,
        string $initContent,
        array $decryptionKeys = [],
        bool $shouldBeMerged = false,
    ): string;
}
