<?php

declare(strict_types=1);

namespace App\Streaming\DRMTechnology\Widevine;

use App\Streaming\DRMTechnology\DRMTechnology;
use App\Streaming\DRMTechnology\Widevine\Classes\DecryptionKeysRetriever;
use App\Streaming\DRMTechnology\Widevine\Classes\SegmentDecryptor;
use App\Streaming\DRMTechnology\Widevine\Classes\PsshRetriever;
use App\Controllers\ManifestController;
use App\Repositories\RedisRepository;
use App\Factory\ObjectFactory;
use App\Streaming\Classes\Episode;
use App\Streaming\Helpers\RequestHelper;

/**
 * Google's DRM Technology
 */
final class Widevine extends DRMTechnology
{
    public string $name = "widevine";
    /**
     * PSSH in Base64
     */
    public string $pssh;

    protected PsshRetriever $psshRetriever;
    protected DecryptionKeysRetriever $decryptionKeysRetriever;
    protected SegmentDecryptor $segmentDecryptor;
    protected RedisRepository $repository;
    protected ManifestController $manifestController;

    public function __construct()
    {
        $this->psshRetriever = ObjectFactory::createPsshRetriever("python");
        $this->decryptionKeysRetriever = ObjectFactory::createDecryptionKeysRetriever(
            "python",
        );
        $this->segmentDecryptor = ObjectFactory::createSegmentDecryptor(
            "shell",
        );
        $this->repository = ObjectFactory::createRepository("redis");
        $this->manifestController = ObjectFactory::createManifestController(
            $this->repository,
        );
    }

    // TODO: Remove this unnecessary method and (just an idea!)
    // check in the database if there are decryption keys for the
    // episode. If there are, the episode is DRM'd, if there are
    // not, this episode is not DRM'd. That'll make it easier to
    // distinguish while keeping the responses fast
    #[\Override]
    public function saveData(
        Episode $episode,
        string $episodeStreamingDrmTechnologyIdentifier,
    ): void {
        // Search for keys in database
        $episode->streamingTechnology->drmTechnology->decryptionKeys = $this->manifestController->getDecryptionKeys(
            $episodeStreamingDrmTechnologyIdentifier,
        );

        if ($episode->streamingTechnology->drmTechnology->decryptionKeys) {
            return;
        }

        // Retrieve the keys instead
        $this->pssh = $this->psshRetriever->getPssh(
            $episode->url,
            $episode->urlHeaders,
            $episode->urlHeaders,
        );

        $episode->streamingTechnology->drmTechnology->decryptionKeys = $this->decryptionKeysRetriever->getDecryptionKeys(
            $this->pssh,
            $episode->streamingTechnology->drmTechnology->licenseUrl,
            $episode->streamingTechnology->drmTechnology->licenseHeaders,
        );

        $this->manifestController->addDecryptionKeys(
            $episodeStreamingDrmTechnologyIdentifier,
            $episode->streamingTechnology->drmTechnology->decryptionKeys,
        );
    }

    private function getInitSegment(
        string $initMediaIdentifier,
        string $reconstructedUrl,
    ): string|null {
        $initContent = RequestHelper::get($reconstructedUrl);

        // // Cleaned from unneeded atoms using bento4
        // $initContent = $this->segmentDecryptor->getDecryptedSegment(
        //     $initContent,
        //     $initContent,
        // );

        $this->manifestController->addInitContent(
            $initMediaIdentifier,
            $initContent,
        );

        return $initContent;
    }

    #[\Override]
    public function getSegment(
        string $episodeStreamingDrmTechnologyIdentifier,
        string $initMediaIdentifier,
        string $reconstructedUrl,
        bool $isInit = false,
    ): string {
        $initContent = $this->manifestController->getInitContent(
            $initMediaIdentifier,
        );

        if ($isInit) {
            if ($initContent) {
                return $initContent;
            }
            return $this->getInitSegment(
                $initMediaIdentifier,
                $reconstructedUrl,
            );
        }

        $decryptionKeys = $this->manifestController->getDecryptionKeys(
            $episodeStreamingDrmTechnologyIdentifier,
        );

        if (!$decryptionKeys) {
            // TODO: Throw error here
            return "";
        }

        // TODO: Add headers
        $segmentContent = RequestHelper::get($reconstructedUrl);

        // If init should be merged with media
        $shouldBeMerged = $initContent === null ? false : true;

        return $this->segmentDecryptor->getDecryptedSegment(
            $segmentContent,
            $initContent,
            $decryptionKeys,
            $shouldBeMerged,
        );
    }
}
