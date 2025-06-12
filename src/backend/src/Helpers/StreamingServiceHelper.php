<?php

declare(strict_types=1);

namespace App\Helpers;

use App\Services\StreamingService;
use App\Services\StreamingServices;
use App\Helpers\RequestHelper;
use App\Services\DecryptionKeysRetrieval;
use App\Services\PsshRetrieval;
use App\Services\ManifestModifier;
use App\Services\SegmentDecryptor;
use App\Repositories\RedisRepository;
use App\Controllers\ManifestController;
use App\Models\ManifestModifier as ModelsManifestModifier;

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

    private array $manifestModifier = [
        "python" => ManifestModifier\PythonBackend::class,
    ];

    private array $segmentDecryptor = [
        "python" => SegmentDecryptor\PythonBackend::class,
        "php" => SegmentDecryptor\Php::class,
        "shell" => SegmentDecryptor\Shell::class,
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

        $manifestModifier = $this->manifestModifier["python"] ?? null;
        $manifestModifier = new $manifestModifier($requestHelper);

        $segmentDecryptor = $this->segmentDecryptor["shell"] ?? null;
        $segmentDecryptor = new $segmentDecryptor();

        // TODO: Add other repositories and make a parent class
        $redisRepository = RedisRepository::class;
        $redisRepository = new $redisRepository();

        $manifestController = ModelsManifestModifier::class;
        $manifestController = new ManifestController($redisRepository);

        return new $service($requestHelper, $psshRetriever, $decryptionKeysRetriever, $manifestModifier, $segmentDecryptor, $manifestController);
    }
}
