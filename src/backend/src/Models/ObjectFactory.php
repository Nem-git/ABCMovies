<?php

declare(strict_types=1);

namespace App\Models;

use App\Controllers\ManifestController;
use App\Models\PsshRetriever;
use App\Models\DecryptionKeysRetriever;
use App\Models\ManifestModifier;
use App\Repositories\RedisRepository;
use App\Services\DecryptionKeysRetriever\PythonBackend as DecryptionKeysRetrieverPythonBackend;
use App\Services\PsshRetriever\PythonBackend as PsshRetrieverPythonBackend;
use App\Services\ManifestModifier\PythonBackend as ManifestModifierPythonBackend;
use App\Services\SegmentDecryptor\Shell as SegmentDecryptorShell;
use App\Services\SegmentDecryptor\Php as SegmentDecryptorPhp;
use App\Services\SegmentDecryptor\PythonBackend as SegmentDecryptorPythonBackend;
use App\Services\StreamingService;
use App\Services\StreamingServices\Toutv as StreamingServieToutv;

class ObjectFactory
{
    private static array $streamingService = [
        "toutv" => StreamingServieToutv::class,
    ];

    private static array $psshRetriever = [
        "python" => PsshRetrieverPythonBackend::class,
    ];

    private static array $decryptionKeysRetriever = [
        "python" => DecryptionKeysRetrieverPythonBackend::class,
    ];

    private static array $manifestModifier = [
        "python" => ManifestModifierPythonBackend::class,
    ];

    private static array $segmentDecryptor = [
        "python" => SegmentDecryptorPythonBackend::class,
        "php" => SegmentDecryptorPhp::class,
        "shell" => SegmentDecryptorShell::class,
    ];

    public static function createStreamingService(string $tag): StreamingService
    {
        $class = self::$streamingService[$tag];
        return new $class();
    }

    public static function createPsshRetriever(string $type): PsshRetriever
    {
        $class = self::$psshRetriever[$type] ?? null;
        return new $class();
    }

    public static function createDecryptionKeysRetriever(string $type): DecryptionKeysRetriever
    {
        $class = self::$decryptionKeysRetriever[$type] ?? null;
        return new $class();
    }

    public static function createManifestModifier(string $type): ManifestModifier
    {
        $class = self::$manifestModifier[$type] ?? null;
        return new $class();
    }

    public static function createSegmentDecryptor(string $type): SegmentDecryptor
    {
        $class = self::$segmentDecryptor[$type] ?? null;
        return new $class();
    }

    public static function createRepository(): RedisRepository
    {
        $class = RedisRepository::class;
        return new $class();
    }

    public static function createManifestController(RedisRepository $repository): ManifestController
    {
        $class = ManifestController::class;
        return new $class($repository);
    }

}
