<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology\Dash;

use App\Streaming\Helpers\RequestHelper;
use App\Streaming\StreamingTechnology\StreamingTechnology;
use App\Streaming\StreamingTechnology\Dash\Helpers\DashHelper;
use App\Streaming\StreamingTechnology\Dash\Classes\ManifestModifier;
use App\Controllers\ManifestController;
use App\Repositories\RedisRepository;
use App\Factory\ObjectFactory;
use App\Streaming\DRMTechnology\DRMTechnology;
use App\Streaming\StreamingTechnology\Helpers\StreamingTechnologyHelper;
use App\Streaming\Classes\Episode;
use App\Streaming\Classes\Season;
use App\Streaming\Classes\Show;

/**
 * Google's adaptative streaming technology
 */
final class Dash extends StreamingTechnology
{
    public string $name = "dash";
    public string $mimeType = "application/dash+xml";
    public DRMTechnology $drmTechnology;

    protected ManifestModifier $manifestModifier;
    protected RedisRepository $repository;
    protected ManifestController $manifestController;

    public function __construct()
    {
        $this->manifestModifier = ObjectFactory::createManifestModifier(
            "go", // python
        );
        $this->repository = ObjectFactory::createRepository("redis");
        $this->manifestController = ObjectFactory::createManifestController(
            $this->repository,
        );
    }

    #[\Override]
    public function getVideo(
        Show $show,
        Season $season,
        Episode $episode,
        array $queryParams = [],
        array $args = [],
    ): string {
        $episodeStreamingDrmTechnologyIdentifier = StreamingTechnologyHelper::getEpisodeStreamingDRMTechnologyIdentifier(
            $show,
            $season,
            $episode,
        );

        // That means that it is not requesting segments, but the manifest
        if (count($args) === 0) {
            return $this->getManifest(
                $episodeStreamingDrmTechnologyIdentifier,
                $episode,
            );
        }

        $dashSegmentCriteria = DashHelper::parseDashSegmentCriteria(
            $queryParams,
            $args,
        );

        if ($dashSegmentCriteria["segmentType"] === "init") {
            return $this->getInitSegment(
                $episode,
                $episodeStreamingDrmTechnologyIdentifier,
                $dashSegmentCriteria["initMediaIdentifier"],
                $dashSegmentCriteria["reconstructedUrl"],
            );
        }

        if ($dashSegmentCriteria["segmentType"] === "media") {
            return $this->getMediaSegment(
                $episode,
                $episodeStreamingDrmTechnologyIdentifier,
                $dashSegmentCriteria["initMediaIdentifier"],
                $dashSegmentCriteria["reconstructedUrl"],
            );
        }

        // Need to throw error
        return "";
    }

    private function getManifest(
        string $episodeStreamingDrmTechnologyIdentifier,
        Episode $episode,
    ): string {
        $manifestContent = RequestHelper::get(
            $episode->url,
            $episode->urlHeaders,
        );

        $modifiedManifestContent = $this->manifestModifier->getModifiedMpd(
            $episode->url,
            $manifestContent,
        );

        if ($episode->containsDrm) {
            $episode->streamingTechnology->drmTechnology->saveData(
                $episode,
                $episodeStreamingDrmTechnologyIdentifier,
            );
        }

        return $modifiedManifestContent;
    }

    private function getInitSegment(
        Episode $episode,
        string $episodeStreamingDrmTechnologyIdentifier,
        string $initMediaIdentifier,
        string $reconstructedUrl,
    ): string|null {
        // If DRM'd
        if ($this->manifestController->getDecryptionKeys(
            $episodeStreamingDrmTechnologyIdentifier,
        )
        ) {
            $initContent = $episode->streamingTechnology->drmTechnology->getSegment(
                $episodeStreamingDrmTechnologyIdentifier,
                $initMediaIdentifier,
                $reconstructedUrl,
                true,
            );
        } else {
            $initContent = RequestHelper::get($reconstructedUrl);
        }

        return $initContent;
    }

    private function getMediaSegment(
        Episode $episode,
        string $episodeStreamingDrmTechnologyIdentifier,
        string $initMediaIdentifier,
        string $reconstructedUrl,
    ): string|null {
        $segmentContent = RequestHelper::get($reconstructedUrl);

        // If DRM'd
        if ($this->manifestController->getDecryptionKeys(
            $episodeStreamingDrmTechnologyIdentifier,
        )
        ) {
            $segmentContent = $episode->streamingTechnology->drmTechnology->getSegment(
                $episodeStreamingDrmTechnologyIdentifier,
                $initMediaIdentifier,
                $reconstructedUrl,
            );
        }

        return $segmentContent;
    }
}
