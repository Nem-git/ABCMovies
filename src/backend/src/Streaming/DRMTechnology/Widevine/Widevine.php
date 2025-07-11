<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology\Widevine;

use App\Streaming\DRMTechnology\DRMTechnology;

/**
 * Google's DRM Technology
 */
class Widevine extends DRMTechnology
{
    public string $name = "widevine";
    /**
     * PSSH in Base64
     */
    public string $pssh;
}
