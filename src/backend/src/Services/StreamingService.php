<?php

declare(strict_types=1);

namespace App\Services;

use App\Controllers\ManifestController;
use Psr\Http\Message\ServerRequestInterface as Request;
use App\Models\Episode;
use App\Models\Season;
use App\Models\Show;
use App\Helpers\RequestHelper;
use App\Models\DecryptionKeysRetriever;
use App\Models\DownloadInfo;
use App\Models\ManifestModifier;
use App\Models\SegmentDecryptor;
use App\Models\ObjectFactory;
use App\Models\PsshRetriever;
use App\Repositories\RedisRepository;
use App\Helpers\StreamingServiceHelper;

require_once __DIR__ . "/../../config/constants.php";

abstract class StreamingService
{
    /**
     * Streaming service's name
     */
    protected string $name;
    /**
     * Streaming service's abreviation (EX: DSNP)
     */
    protected string $tag;

    protected PsshRetriever $psshRetriever;
    protected DecryptionKeysRetriever $decryptionKeysRetriever;
    protected ManifestModifier $manifestModifier;
    protected SegmentDecryptor $segmentDecryptor;
    protected RedisRepository $repository;
    protected ManifestController $manifestController;

    public function __construct()
    {
        $this->psshRetriever = ObjectFactory::createPsshRetriever("python");
        $this->decryptionKeysRetriever = ObjectFactory::createDecryptionKeysRetriever("python");
        $this->manifestModifier = ObjectFactory::createManifestModifier("python");
        $this->segmentDecryptor = ObjectFactory::createSegmentDecryptor("shell");
        $this->repository = ObjectFactory::createRepository();
        $this->manifestController = ObjectFactory::createManifestController($this->repository);
    }

    //region Parsing

    abstract protected function parseSearchResults(array $response): array;
    abstract protected function parseShowInfo(Show $show, array $response): void;
    abstract protected function parseSeasonInfo(Season $season, array $response): void;
    abstract protected function parseEpisodeInfo(Episode $episode, array $response): void;
    abstract protected function parseEpisodeDownloadInfo(Episode $episode, array $response): DownloadInfo;

    //endregion

    //region Get Informations

    //region Search

    public function getSearchResults(Request $request, array $args): array
    {
        $searchCriteria = StreamingServiceHelper::parseSearchCriteria($request, $args);

        return $this->executeSearch(
            $searchCriteria["query"],
            $searchCriteria["amount"],
        );
    }

    public function executeSearch(string $query, int $amount): array
    {
        $parameters = $this->getSearchParameters($query, $amount);
        $response = RequestHelper::get($this->getSearchUrl(), HTTP_DEFAULT_HEADERS, $parameters);
        $searchResults = $this->parseSearchResults(json_decode($response, true));
        return $searchResults;
    }

    //endregion

    //region Show

    public function getShowInfo(Request $request, array $args): Show
    {
        $showInfoCriteria = StreamingServiceHelper::parseShowInfoCriteria($request, $args);

        return $this->executeShowInfo(
            $showInfoCriteria["showId"],
        );
    }

    public function executeShowInfo(string $showId): Show
    {
        $show = new Show();
        $show->id = $showId;

        $response = RequestHelper::get($this->getShowInfoUrl($showId), HTTP_DEFAULT_HEADERS, $this->getShowInfoParameters());
        $this->parseShowInfo($show, json_decode($response, true));
        return $show;
    }

    //endregion

    //region Season

    public function getSeasonInfo(Request $request, array $args): Season
    {
        $seasonInfoCriteria = StreamingServiceHelper::parseSeasonInfoCriteria($request, $args);

        return $this->executeSeasonInfo(
            $seasonInfoCriteria["showId"],
            $seasonInfoCriteria["seasonId"],
        );
    }

    public function executeSeasonInfo(string $showId, string $seasonId): Season
    {
        $season = new Season();
        $season->id = $seasonId;

        $response = RequestHelper::get($this->getSeasonInfoUrl($showId, $seasonId), HTTP_DEFAULT_HEADERS, $this->getSeasonInfoParameters());
        $this->parseSeasonInfo($season, json_decode($response, true));
        return $season;
    }

    //endregion

    //region Episode

    public function getEpisodeInfo(Request $request, array $args): Episode
    {
        $episodeInfoCriteria = StreamingServiceHelper::parseEpisodeInfoCriteria($request, $args);

        return $this->executeEpisodeInfo(
            $episodeInfoCriteria["showId"],
            $episodeInfoCriteria["seasonId"],
            $episodeInfoCriteria["episodeId"],
        );
    }

    public function executeEpisodeInfo(string $showId, string $seasonId, string $episodeId): Episode
    {
        $episode = new Episode();
        $episode->id = $episodeId;
        $episode->url = StreamingServiceHelper::getStreamUrl($this->tag, $showId, $seasonId, $episodeId, "dash");

        $response = RequestHelper::get($this->getEpisodeInfoUrl($showId, $seasonId, $episodeId), HTTP_DEFAULT_HEADERS, $this->getEpisodeInfoParameters());
        $this->parseEpisodeInfo($episode, json_decode($response, true));

        return $episode;
    }

    //endregion

    //region Episode's Stream

    public function getEpisodeStream(Request $request, array $args): string
    {
        $episodeStreamCriteria = StreamingServiceHelper::parseEpisodeStreamCriteria($request, $args);

        return $this->executeEpisodeStream(
            $episodeStreamCriteria["showId"],
            $episodeStreamCriteria["seasonId"],
            $episodeStreamCriteria["episodeId"],
        );
    }

    public function executeEpisodeStream(string $showId, string $seasonId, string $episodeId): string
    {
        $episode = $this->executeEpisodeInfo($showId, $seasonId, $episodeId);

        $response = json_decode(RequestHelper::get($this->getEpisodeInfoUrl($showId, $seasonId, $episodeId), HTTP_DEFAULT_HEADERS, $this->getEpisodeInfoParameters()), true);
        $downloadInfo = $this->parseEpisodeDownloadInfo($episode, $response);

        $this->psshRetriever->getPssh($downloadInfo);
        $this->decryptionKeysRetriever->getDecryptionKeys($downloadInfo);

        $episodeDatabaseIdentifier = StreamingServiceHelper::getEpisodeDatabaseIdentifier($this->tag, $showId, $seasonId, $episodeId);
        $this->manifestController->addDecryptionKeys($episodeDatabaseIdentifier, $downloadInfo->decryptionKeys);

        $modifiedManifestContent = $this->manifestModifier->getModifiedMpd($downloadInfo);

        return $modifiedManifestContent;
    }

    //endregion

    //region Episode's Init Segments

    public function getEpisodeInitSegment(Request $request, array $args): string
    {
        $episodeInitSegmentCriteria = StreamingServiceHelper::parseEpisodeInitSegmentCriteria($request, $args);

        return $this->executeEpisodeInitSegment(
            $episodeInitSegmentCriteria["originalInitUrl"],
        );
    }

    public function executeEpisodeInitSegment(string $originalInitUrl): string
    {
        $initContent = $this->manifestController->getInitContent($originalInitUrl);

        if (!$initContent) {
            $initContent = RequestHelper::get($originalInitUrl);
            $this->manifestController->addInitContent($originalInitUrl, $initContent);
        }

        return $initContent;
    }

    //endregion

    //region Episode's Media Segments

    public function getEpisodeMediaSegment(Request $request, array $args): string
    {
        $episodeMediaSegmentCriteria = StreamingServiceHelper::parseEpisodeMediaSegmentCriteria($request, $args);

        return $this->executeEpisodeMediaSegment(
            $episodeMediaSegmentCriteria["originalInitUrl"],
            $episodeMediaSegmentCriteria["originalMediaUrl"],
            $episodeMediaSegmentCriteria["showId"],
            $episodeMediaSegmentCriteria["seasonId"],
            $episodeMediaSegmentCriteria["episodeId"],
        );
    }

    public function executeEpisodeMediaSegment(string $originalInitUrl, string $originalMediaUrl, string $showId, string $seasonId, string $episodeId): string
    {
        $episodeDatabaseIdentifier = StreamingServiceHelper::getEpisodeDatabaseIdentifier($this->tag, $showId, $seasonId, $episodeId);
        $decryptionKeys = $this->manifestController->getDecryptionKeys($episodeDatabaseIdentifier);

        $initContent = $this->manifestController->getInitContent($originalInitUrl);

        $segmentContent = RequestHelper::get($originalMediaUrl);
        $decryptedSegmentContent = $this->segmentDecryptor->getDecryptedSegment($initContent, $segmentContent, $decryptionKeys);

        return $decryptedSegmentContent;
    }

    //endregion

    //endregion

    //region Abstract methods for URLs and parameters (to be implemented per service)

    abstract protected function getSearchUrl(): string;
    abstract protected function getSearchParameters(string $query, int $amount): array;

    abstract protected function getShowInfoUrl(string $showId): string;
    abstract protected function getShowInfoParameters(): array;

    abstract protected function getSeasonInfoUrl(string $showId, string $seasonId): string;
    abstract protected function getSeasonInfoParameters(): array;

    abstract protected function getEpisodeInfoUrl(string $showId, string $seasonId, string $episodeId): string;
    abstract protected function getEpisodeInfoParameters(): array;

    //endregion

}
