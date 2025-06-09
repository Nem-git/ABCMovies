<?php

declare(strict_types=1);

namespace App\Helpers;

use App\Services\StreamingService;
use App\Services\StreamingServices;
use App\Helpers\RequestHelper;
use App\Services\PsshRetrieval;
use App\Services\DecryptionKeysRetrieval;

class StreamingServiceHelper
{
    private array $services = [
        "toutv" => StreamingServices\Toutv::class,
        "noovo" => StreamingServices\Toutv::class, // TODO: Add these other services
        "crave" => StreamingServices\Toutv::class,
    ];

    private array $psshRetriever = [
        "python" => PsshRetrieval\PythonBackend::class,
    ];

    private array $decryptionKeysRetriever = [
        "python" => DecryptionKeysRetrieval\PythonBackend::class,
    ];

    public function pick(string $name): ?StreamingService
    {
        $service = $this->services[$name] ?? null;

        $requestHelper = RequestHelper::class;
        $requestHelper = new $requestHelper();

        $psshRetriever = $this->psshRetriever["python"] ?? null;
        $psshRetriever = new $psshRetriever($requestHelper);

        $decryptionKeysRetriever = $this->decryptionKeysRetriever["python"] ?? null;
        $decryptionKeysRetriever = new $decryptionKeysRetriever($requestHelper);


        return new $service($requestHelper, $psshRetriever, $decryptionKeysRetriever);
    }
}
