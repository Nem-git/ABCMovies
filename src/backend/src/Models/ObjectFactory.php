<?php

declare(strict_types=1);

namespace App\Models;

use App\Controllers\{
    ManifestController,
};
use App\Models\{
    DecryptionKeysRetriever,
    DownloadInfo,
    Episode,
    ManifestModifier,
    MediaRecommender,
    PsshRetriever,
    SearchRecommender,
    Season,
    Show,
    StreamingTechnology
};
use App\Repositories\{
    RedisRepository,
};
use App\Services\{
    StreamingService,
    StreamingServiceManager,
};
use App\Services\ManifestModifier\{
    PythonBackend as ManifestModifierPythonBackend,
};
use App\Services\MediaRecommender\{
    Random as MediaRecommenderRandom
};
use App\Services\PsshRetriever\{
    PythonBackend as PsshRetrieverPythonBackend,
};
use App\Services\DecryptionKeysRetriever\{
    PythonBackend as DecryptionKeysRetrieverPythonBackend,
};
use App\Services\SearchRecommender\{
    Fuzzy as SearchRecommenderFuzzy
};
use App\Services\SegmentDecryptor\{
    Php as SegmentDecryptorPhp,
    PythonBackend as SegmentDecryptorPythonBackend,
    Shell as SegmentDecryptorShell
};
use App\Services\StreamingServices\{
    Toutv as StreamingServiceToutv,
};
use App\Services\StreamingTechnology\{
    Dash as StreamingTechnologyDash,
    Hls as StreamingTechnologyHls,
    Mp4 as StreamingTechnologyMp4
};

class ObjectFactory
{
    private static array $streamingService = [
        "TOUTV" => StreamingServiceToutv::class,
    ];

    private static array $streamingTechnology = [
        "dash" => StreamingTechnologyDash::class,
        "hls" => StreamingTechnologyHls::class,
        "mp4" => StreamingTechnologyMp4::class,
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

    private static array $repository = [
        "redis" => RedisRepository::class,
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

    public static function createStreamingTechnology(string $name): StreamingTechnology
    {
        $class = self::$streamingTechnology[$name];
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

    public static function createRepository(string $type): RedisRepository
    {
        $class = self::$repository[$type] ?? null;
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
