<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology\Widevine\Classes;

abstract class DecryptionKeysRetriever
{
    abstract public function getDecryptionKeys(
        string $pssh,
        string $licenseUrl,
        array $licenseHeaders,
    ): array;
}
