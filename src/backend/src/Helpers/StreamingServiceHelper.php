<?php

declare(strict_types=1);

namespace App\Helpers;

use App\Services\StreamingService;
use App\Services\StreamingServices as StreamingServices;
use App\Helpers\RequestHelper as RequestHelper;
use App\Services\DrmServices;

class StreamingServiceHelper
{
    private array $services = [
        "toutv" => StreamingServices\Toutv::class,
        "noovo" => StreamingServices\Toutv::class,
        "crave" => StreamingServices\Toutv::class,
    ];

    private array $drmServices = [
        "widevine" => DrmServices\Python::class,
    ];

    public function pick(string $name): ?StreamingService
    {
        $service = $this->services[$name] ?? null;

        $requestHelper = RequestHelper::class;
        $requestHelper = new $requestHelper();

        $widevineDrmService = $this->drmServices["widevine"] ?? null;
        $widevineDrmService = new $widevineDrmService($requestHelper);

        return new $service($requestHelper, $widevineDrmService);
    }
}
