<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology;

class DRMTechnology
{
    public string $name;

    /**
     * License Url
     */
    public string $licenseUrl;

    /**
     * Headers for the license request
     */
    public array $licenseHeaders = [];

    /**
     * Video's decryption keys
     */
    public array $decryptionKeys;
}
