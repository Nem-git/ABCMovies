<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology\Dash;

use Psr\Http\Message\ServerRequestInterface as Request;
use App\Streaming\Helpers\RequestHelper;
use App\Streaming\StreamingTechnology\StreamingTechnology;
use App\Streaming\StreamingService\StreamingService;
use App\Streaming\StreamingService\Helpers\StreamingServiceHelper;
use App\Streaming\StreamingTechnology\Dash\Helpers\DashHelper;
use App\Streaming\DRMTechnology\Widevine\Classes\DecryptionKeysRetriever;
use App\Streaming\StreamingTechnology\Dash\Classes\ManifestModifier;
use App\Streaming\DRMTechnology\Widevine\Classes\SegmentDecryptor;
use App\Streaming\DRMTechnology\Widevine\Classes\PsshRetriever;
use App\Controllers\ManifestController;
use App\Repositories\RedisRepository;
use App\Factory\ObjectFactory;
use App\Streaming\DRMTechnology\DRMTechnology;

/**
 * Google's adaptative streaming technology
 */
class Dash extends StreamingTechnology
{
    public string $name = "dash";
    public string $mimeType = "application/dash+xml";
    public DRMTechnology $drmTechnology;

    protected PsshRetriever $psshRetriever;
    protected DecryptionKeysRetriever $decryptionKeysRetriever;
    protected ManifestModifier $manifestModifier;
    protected SegmentDecryptor $segmentDecryptor;
    protected RedisRepository $repository;
    protected ManifestController $manifestController;

    public function __construct()
    {
        $this->psshRetriever = ObjectFactory::createPsshRetriever("python");
        $this->decryptionKeysRetriever = ObjectFactory::createDecryptionKeysRetriever(
            "python",
        );
        $this->manifestModifier = ObjectFactory::createManifestModifier(
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

    public function getVideo(
        StreamingService $streamingService,
        Request $request,
        string $showId,
        string $seasonId,
        string $episodeId,
        array $args = [],
    ): string {
        // That means that it is not requesting segments, but the manifest
        if (count($args) === 0) {
            return $this->getManifest(
                $streamingService,
                $request,
                $showId,
                $seasonId,
                $episodeId,
            );
        }

        $segmentType = strtolower(array_shift($args));
        $dashSegmentCriteria = DashHelper::parseDashSegmentCriteria(
            $request,
            $args,
        );

        if ($segmentType === "init") {
            return $this->getInitSegment(
                $dashSegmentCriteria["initMediaIdentifier"],
                $dashSegmentCriteria["reconstructedUrl"],
            );
        }

        if ($segmentType === "media") {
            return $this->getMediaSegment(
                $streamingService,
                $request,
                $showId,
                $seasonId,
                $episodeId,
                $dashSegmentCriteria["initMediaIdentifier"],
                $dashSegmentCriteria["reconstructedUrl"],
            );
        }

        // Need to throw error
        return "";
    }

    private function getManifest(
        StreamingService $streamingService,
        Request $request,
        string $showId,
        string $seasonId,
        string $episodeId,
    ): string {
        $episode = $streamingService->executeEpisodeInfo(
            $showId,
            $seasonId,
            $episodeId,
        );

        $streamingService->getEpisodeStreamInfo($episode);

        // TODO: Find a good way to retrieve the right headers
        $pssh = $this->psshRetriever->getPssh($episode->url, [], []);

        // Create the database id to identify the Decryption Keys
        $episodeDatabaseIdentifier = StreamingServiceHelper::getEpisodeDatabaseIdentifier(
            $streamingService->tag,
            $showId,
            $seasonId,
            $episodeId,
        );

        $episode->streamingTechnology->drmTechnology->decryptionKeys = $this->manifestController->getDecryptionKeys(
            $episodeDatabaseIdentifier,
        );

        // Retrieve the keys instead of relying on the DB
        if (!$episode->streamingTechnology->drmTechnology->decryptionKeys) {
            $episode->streamingTechnology->drmTechnology->decryptionKeys = $this->decryptionKeysRetriever->getDecryptionKeys(
                $pssh,
                $episode->streamingTechnology->drmTechnology->licenseUrl,
                $episode->streamingTechnology->drmTechnology->licenseHeaders,
            );

            $this->manifestController->addDecryptionKeys(
                $episodeDatabaseIdentifier,
                $episode->streamingTechnology->drmTechnology->decryptionKeys,
            );
        }

        $manifestContent = RequestHelper::get($episode->url);

        $modifiedManifestContent = $this->manifestModifier->getModifiedMpd(
            $episode->url,
            $manifestContent,
        );

        return $modifiedManifestContent;
    }

    private function getInitSegment(
        string $initMediaIdentifier,
        string $reconstructedUrl,
    ): string {
        $initContent = $this->manifestController->getInitContent(
            $reconstructedUrl,
        );

        if (!$initContent) {
            $initContent = RequestHelper::get($reconstructedUrl);
            $this->manifestController->addInitContent(
                $initMediaIdentifier,
                $initContent,
            );
        }

        return $initContent;
    }

    private function getMediaSegment(
        StreamingService $streamingService,
        Request $request,
        string $showId,
        string $seasonId,
        string $episodeId,
        string $initMediaIdentifier,
        string $reconstructedUrl,
    ): string {
        $episodeDatabaseIdentifier = StreamingServiceHelper::getEpisodeDatabaseIdentifier(
            $streamingService->tag,
            $showId,
            $seasonId,
            $episodeId,
        );

        $decryptionKeys = $this->manifestController->getDecryptionKeys(
            $episodeDatabaseIdentifier,
        );

        if (!$decryptionKeys) {
            $this->getManifest(
                $streamingService,
                $request,
                $showId,
                $seasonId,
                $episodeId,
            );
            $decryptionKeys = $this->manifestController->getDecryptionKeys(
                $episodeDatabaseIdentifier,
            );
        }

        return $this->executeMediaSegment(
            $initMediaIdentifier,
            $reconstructedUrl,
            $decryptionKeys,
        );
    }

    private function executeMediaSegment(
        string $initMediaIdentifier,
        string $reconstructedUrl,
        array $decryptionKeys,
    ): string {
        $initContent = $this->manifestController->getInitContent(
            $initMediaIdentifier,
        );

        // Shouldn't happen with video players that are not dumb
        if (!$initContent) {
            // Need to add error
            return "";
        }

        $segmentContent = RequestHelper::get($reconstructedUrl);
        $decryptedSegmentContent = $this->segmentDecryptor->getDecryptedSegment(
            $initContent,
            $segmentContent,
            $decryptionKeys,
        );

        return $decryptedSegmentContent;
    }
}
