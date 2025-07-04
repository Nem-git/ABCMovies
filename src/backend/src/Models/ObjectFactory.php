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
use App\Services\StreamingServices\Toutv as StreamingServiceToutv;
use App\Models\SearchRecommender;
use App\Services\SearchRecommender\Fuzzy as SearchRecommenderFuzzy;
use App\Models\MediaRecommender;
use App\Services\MediaRecommender\Random as MediaRecommenderRandom;
use App\Services\StreamingServiceManager;
use App\Models\Show;
use App\Models\Season;
use App\Models\Episode;
use App\Models\DownloadInfo;

class ObjectFactory
{
    private static array $streamingService = [
        "TOUTV" => StreamingServiceToutv::class,
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

    private static array $searchRecommender = [
        "fuzzy" => SearchRecommenderFuzzy::class,
    ];

    private static array $mediaRecommender = [
        "random" => MediaRecommenderRandom::class,
    ];

    public static function createStreamingService(string $tag): StreamingService
    {
        $class = self::$streamingService[$tag];
        return new $class();
    }

    public static function createShow(): Show
    {
        $class = Show::class;
        return new $class();
    }

    public static function createSeason(): Season
    {
        $class = Season::class;
        return new $class();
    }

    public static function createEpisode(): Episode
    {
        $class = Episode::class;
        return new $class();
    }

    public static function createDownloadInfo(): DownloadInfo
    {
        $class = DownloadInfo::class;
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

    public static function createSearchRecommender(string $type): SearchRecommender
    {
        $class = self::$searchRecommender[$type] ?? null;
        return new $class();
    }

    public static function createMediaRecommender(string $type): MediaRecommender
    {
        $class = self::$mediaRecommender[$type] ?? null;
        return new $class();
    }

    public static function createStreamingServiceManager(): StreamingServiceManager
    {
        $class = StreamingServiceManager::class;
        return new $class();
    }

}
