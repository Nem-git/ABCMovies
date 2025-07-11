<?php

declare(strict_types=1);

namespace App\Factory;

class ObjectFactory
{
    private static array $streamingService = [
        "TOUTV" => \App\Streaming\StreamingService\Toutv\Toutv::class,
    ];

    private static array $streamingTechnology = [
        "dash" => \App\Streaming\StreamingTechnology\Dash\Dash::class,
        "hls" => \App\Streaming\StreamingTechnology\Hls\Hls::class,
        "mp4" => \App\Streaming\StreamingTechnology\Mp4\Mp4::class,
        "smooth" => \App\Streaming\StreamingTechnology\Smooth\Smooth::class,
    ];

    private static array $psshRetriever = [
        "python" => \App\Streaming\DRMTechnology\Widevine\PsshRetriever\PythonBackend::class,
    ];

    private static array $decryptionKeysRetriever = [
        "python" => \App\Streaming\DRMTechnology\Widevine\DecryptionKeysRetriever\PythonBackend::class,
    ];

    private static array $manifestModifier = [
        "python" => \App\Streaming\StreamingTechnology\Dash\ManifestModifier\PythonBackend::class,
    ];

    private static array $segmentDecryptor = [
        "python" => \App\Streaming\DRMTechnology\Widevine\SegmentDecryptor\PythonBackend::class,
        "php" => \App\Streaming\DRMTechnology\Widevine\SegmentDecryptor\PHP::class,
        "shell" => \App\Streaming\DRMTechnology\Widevine\SegmentDecryptor\Shell::class,
    ];

    private static array $repository = [
        "redis" => \App\Repositories\RedisRepository::class,
    ];

    private static array $searchRecommender = [
        "fuzzy" => \App\Streaming\StreamingServiceManager\SearchRecommender\Fuzzy::class,
    ];

    private static array $mediaRecommender = [
        "random" => \App\Streaming\StreamingServiceManager\MediaRecommender\Random::class,
    ];

    public static function createStreamingService(string $tag): \App\Streaming\StreamingService\StreamingService
    {
        $class = self::$streamingService[$tag] ?? null;
        if ($class === null) {
            throw new \InvalidArgumentException("Unknown streaming service tag: $tag");
        }
        return new $class();
    }

    public static function createStreamingTechnology(string $name): \App\Streaming\StreamingTechnology\StreamingTechnology
    {
        $class = self::$streamingTechnology[$name] ?? null;
        if ($class === null) {
            throw new \InvalidArgumentException("Unknown streaming technology name: $name");
        }
        return new $class();
    }

    public static function createShow(): \App\Streaming\Classes\Show
    {
        return new \App\Streaming\Classes\Show();
    }

    public static function createSeason(): \App\Streaming\Classes\Season
    {
        return new \App\Streaming\Classes\Season();
    }

    public static function createEpisode(): \App\Streaming\Classes\Episode
    {
        return new \App\Streaming\Classes\Episode();
    }

    public static function createDownloadInfo(): \App\Streaming\Classes\DownloadInfo
    {
        return new \App\Streaming\Classes\DownloadInfo();
    }

    public static function createPsshRetriever(string $type): \App\Streaming\DRMTechnology\Widevine\Classes\PsshRetriever
    {
        $class = self::$psshRetriever[$type] ?? null;
        if ($class === null) {
            throw new \InvalidArgumentException("Unknown PSSH retriever type: $type");
        }
        return new $class();
    }

    public static function createDecryptionKeysRetriever(string $type): \App\Streaming\DRMTechnology\Widevine\Classes\DecryptionKeysRetriever
    {
        $class = self::$decryptionKeysRetriever[$type] ?? null;
        if ($class === null) {
            throw new \InvalidArgumentException("Unknown decryption keys retriever type: $type");
        }
        return new $class();
    }

    public static function createManifestModifier(string $type): \App\Streaming\StreamingTechnology\Dash\Classes\ManifestModifier
    {
        $class = self::$manifestModifier[$type] ?? null;
        if ($class === null) {
            throw new \InvalidArgumentException("Unknown manifest modifier type: $type");
        }
        return new $class();
    }

    public static function createSegmentDecryptor(string $type): \App\Streaming\DRMTechnology\Widevine\Classes\SegmentDecryptor
    {
        $class = self::$segmentDecryptor[$type] ?? null;
        if ($class === null) {
            throw new \InvalidArgumentException("Unknown segment decryptor type: $type");
        }
        return new $class();
    }

    public static function createRepository(string $type): \App\Repositories\RedisRepository
    {
        $class = self::$repository[$type] ?? null;
        if ($class === null) {
            throw new \InvalidArgumentException("Unknown repository type: $type");
        }
        return new $class();
    }

    public static function createManifestController(\App\Repositories\RedisRepository $repository): \App\Controllers\ManifestController
    {
        return new \App\Controllers\ManifestController($repository);
    }

    public static function createSearchRecommender(string $type): \App\Streaming\StreamingServiceManager\Classes\SearchRecommender
    {
        $class = self::$searchRecommender[$type] ?? null;
        if ($class === null) {
            throw new \InvalidArgumentException("Unknown search recommender type: $type");
        }
        return new $class();
    }

    public static function createMediaRecommender(string $type): \App\Streaming\StreamingServiceManager\Classes\MediaRecommender
    {
        $class = self::$mediaRecommender[$type] ?? null;
        if ($class === null) {
            throw new \InvalidArgumentException("Unknown media recommender type: $type");
        }
        return new $class();
    }

    public static function createStreamingServiceManager(): \App\Streaming\StreamingServiceManager\StreamingServiceManager
    {
        return new \App\Streaming\StreamingServiceManager\StreamingServiceManager();
    }

}
