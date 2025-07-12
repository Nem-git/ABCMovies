<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology;

use App\Streaming\Classes\Episode;

abstract class DRMTechnology
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

    abstract public function saveData(
        Episode $episode,
        string $episodeStreamingDrmTechnologyIdentifier,
    ): void;

    abstract public function getSegment(
        string $episodeStreamingDrmTechnologyIdentifier,
        string $initMediaIdentifier,
        string $reconstructedUrl,
        bool $isInit = false,
    ): string;
}
