<?php

declare(strict_types=1);

namespace App\Models;

class DownloadInfo
{
    /**
     * MPD Url
     */
    public string $mpdUrl;

    /**
     * MPD Content
     */
    public string $mpdContent;

    /**
     * Headers for the MPD request
     */
    public array $mpdHeaders = [];

    /**
     * PSSH in Base64
     */
    public string $pssh;

    /**
     * License Url
     */
    public string $licenseUrl;

    /**
     * Headers for the license request
     */
    public array $licenseHeaders = [];

    /**
     * Headers for the segments requests
     */
    public array $segmentsHeaders = [];

    /**
     * Decryption keys for the content of the MPD
     */
    public array $decryptionKeys;
}
