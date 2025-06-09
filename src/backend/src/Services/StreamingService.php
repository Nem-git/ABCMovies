<?php

declare(strict_types=1);

namespace App\Services;

use Psr\Http\Message\ServerRequestInterface as Request;
use Psr\Http\Message\ResponseInterface as Response;
use App\Models\Episode;
use App\Models\Season;
use App\Models\Show;
use App\Helpers\RequestHelper;
use App\Models\DecryptionKeysRetrieval;
use App\Models\DownloadInfo;
use App\Models\PsshRetrieval;

require_once __DIR__ . "/../../config/constants.php"; // TODO: Verify if that's actually a good way to do it

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

    protected RequestHelper $request;
    protected PsshRetrieval $pssh;
    protected DecryptionKeysRetrieval $decryptionKeys;

    public function __construct(RequestHelper $requestHelper, PsshRetrieval $psshRetrieval, DecryptionKeysRetrieval $decryptionKeysRetrieval)
    {
        $this->request = $requestHelper;
        $this->pssh = $psshRetrieval;
        $this->decryptionKeys = $decryptionKeysRetrieval;
    }

    //region Parsing

    abstract protected function parseSearchResults(array $ssResponse): array;
    abstract protected function parseShowInfo(Show $show, array $ssResponse): void;
    abstract protected function parseSeasonInfo(Season $season, array $ssResponse): void;
    abstract protected function parseEpisodeInfo(Episode $episode, array $ssResponse): void;
    abstract protected function parseEpisodeDownloadInfo(Episode $episode, array $ssResponse): DownloadInfo;

    //endregion


    //region Get Informations

    // I don't give it a definition because it depends on the streamingservice
    // (Mostly to give the ability for streaming services to respect the class)
    // because they would all create different functions for everything, while this is same for everyone
    abstract public function getEpisodeDownloadInfoOptional(Episode $episode, DownloadInfo $downloadInfo): void;

    public function getSearchResults(Request $request, Response $response, array $args): array
    {
        $query = $args["query"] ?? "*";
        $amount = (int)($request->getQueryParams()["amount"] ?? 20);

        $parameters = $this->getSearchParameters($query, $amount);

        $ssResponse = $this->request->get($this->getSearchUrl(), HTTP_DEFAULT_HEADERS, $parameters);
        return $this->parseSearchResults(json_decode($ssResponse, true));
    }

    public function getShowInfo(Request $request, Response $response, array $args): Show
    {
        $showId = $args["show"] ?? "";
        $show = new Show();
        $show->id = $showId;

        $ssResponse = $this->request->get($this->getShowInfoUrl($showId), HTTP_DEFAULT_HEADERS, $this->getShowInfoParameters());
        $this->parseShowInfo($show, json_decode($ssResponse, true));

        return $show;
    }

    public function getSeasonInfo(Request $request, Response $response, array $args): Season
    {
        $showId = $args["show"] ?? "";
        $seasonId = $args["season"] ?? "";
        $season = new Season();
        $season->id = $seasonId;

        $ssResponse = $this->request->get($this->getSeasonInfoUrl($showId, $seasonId), HTTP_DEFAULT_HEADERS, $this->getSeasonInfoParameters());
        $this->parseSeasonInfo($season, json_decode($ssResponse, true));

        return $season;
    }

    public function getEpisodeInfo(Request $request, Response $response, array $args, bool $returnLastRequest = false): Episode | array
    {
        $showId = $args["show"] ?? "";
        $seasonId = $args["season"] ?? "";
        $episodeId = $args["episode"] ?? "";
        $episode = new Episode();
        $episode->id = $episodeId;
        $episode->url = $request->getUri() . "/manifest.mpd"; // TODO: Improve the link creation, this is wrong

        // TODO: Add verifications to make sure the request has a valid output
        $ssResponse = $this->request->get($this->getEpisodeInfoUrl($showId, $seasonId, $episodeId), HTTP_DEFAULT_HEADERS, $this->getEpisodeInfoParameters());
        $this->parseEpisodeInfo($episode, json_decode($ssResponse, true));

        return $returnLastRequest ? [$episode, json_decode($ssResponse, true)] : $episode;
    }

    public function getEpisodeDownloadInfo(Request $request, Response $response, array $args): DownloadInfo
    {
        list($episode, $ssResponse) = $this->getEpisodeInfo($request, $response, $args, true);
        $downloadInfo = $this->parseEpisodeDownloadInfo($episode, $ssResponse);

        $this->getEpisodeDownloadInfoOptional($episode, $downloadInfo);

        $this->pssh->getPssh($downloadInfo);
        $this->decryptionKeys->getDecryptionKeys($downloadInfo);

        return $downloadInfo;
    }

    public function getEpisodeManifest(Request $request, Response $response, array $args): string
    {
        $downloadInfo = $this->getEpisodeDownloadInfo($request, $response, $args); // Because we need the MPD url and headers

        return $this->request->get($downloadInfo->mpdUrl, $downloadInfo->mpdHeaders);
    }

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
