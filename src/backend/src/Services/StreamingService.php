<?php

declare(strict_types=1);

namespace App\Services;

use App\Controllers\ManifestController;
use Psr\Http\Message\ServerRequestInterface as Request;
use Psr\Http\Message\ResponseInterface as Response;
use App\Models\Episode;
use App\Models\Season;
use App\Models\Show;
use App\Helpers\RequestHelper;
use App\Helpers\SlimResponseHelper;
use App\Models\DecryptionKeysRetriever;
use App\Models\DownloadInfo;
use App\Models\ManifestModifier;
use App\Models\SegmentDecryptor;
use App\Models\ObjectFactory;
use App\Models\PsshRetriever;
use App\Repositories\RedisRepository;

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

    //region Functions that return to public

    public function getSearchResults(Request $request, Response $response, array $args): Response
    {
        $query = $args["query"] ?? "*";
        $amount = (int)($request->getQueryParams()["amount"] ?? 20);

        $parameters = $this->getSearchParameters($query, $amount);

        $ssResponse = RequestHelper::get($this->getSearchUrl(), HTTP_DEFAULT_HEADERS, $parameters);

        $searchResults = $this->parseSearchResults(json_decode($ssResponse, true));
        return SlimResponseHelper::response_json($searchResults, $response);
    }

    public function getShowInfo(Request $request, Response $response, array $args): Response
    {
        $showId = $args["show"] ?? "";
        $show = new Show();
        $show->id = $showId;

        $ssResponse = RequestHelper::get($this->getShowInfoUrl($showId), HTTP_DEFAULT_HEADERS, $this->getShowInfoParameters());
        $this->parseShowInfo($show, json_decode($ssResponse, true));

        return SlimResponseHelper::response_json($show, $response);
    }

    public function getSeasonInfo(?Request $request, Response $response, array $args): Response
    {
        $showId = $args["show"] ?? "";
        $seasonId = $args["season"] ?? "";
        $season = new Season();
        $season->id = $seasonId;

        $ssResponse = RequestHelper::get($this->getSeasonInfoUrl($showId, $seasonId), HTTP_DEFAULT_HEADERS, $this->getSeasonInfoParameters());
        $this->parseSeasonInfo($season, json_decode($ssResponse, true));

        return SlimResponseHelper::response_json($season, $response);
    }

    public function getEpisodeInfo(?Request $request, ?Response $response, array $args, bool $returnLastRequest = false): Response | array
    {
        $showId = $args["show"] ?? "";
        $seasonId = $args["season"] ?? "";
        $episodeId = $args["episode"] ?? "";
        $episode = new Episode();
        $episode->id = $episodeId;
        $episode->url = $request->getUri() . "/manifest.mpd"; // TODO: Improve the link creation, this is wrong

        // TODO: Add verifications to make sure the request has a valid output
        $ssResponse = RequestHelper::get($this->getEpisodeInfoUrl($showId, $seasonId, $episodeId), HTTP_DEFAULT_HEADERS, $this->getEpisodeInfoParameters());
        $this->parseEpisodeInfo($episode, json_decode($ssResponse, true));

        $returnLastRequest ?: $episode;

        return $returnLastRequest ? [$episode, json_decode($ssResponse, true)] : SlimResponseHelper::response_json($episode, $response);
    }

    public function getEpisodeManifest(Request $request, Response $response, array $args): Response
    {
        $downloadInfo = $this->getEpisodeDownloadInfo($request, $args); // Because we need the MPD url and headers

        $modifiedManifestContent = $this->manifestModifier->getModifiedMpd($downloadInfo);

        return SlimResponseHelper::response_dash($modifiedManifestContent, $response);
    }


    public function getEpisodeInitSegment(Request $request, Response $response, array $args): Response
    {
        $encodedBaseUrl = $args["encodedBaseUrl"];
        $segmentPath = $args["segmentPath"];
        $queryParameters = $request->getQueryParams();

        $originalUrl = base64_decode($encodedBaseUrl, true);
        $originalUrl .= $segmentPath;

        $initContent = $this->manifestController->getInitContent($originalUrl . RequestHelper::format_parameters($queryParameters));

        if ($initContent) {
            return SlimResponseHelper::response_segment($initContent, $response);
        }

        $initContent = RequestHelper::get($originalUrl, parameters: $queryParameters); // TODO: Add segments headers

        $this->manifestController->addInitContent($originalUrl . RequestHelper::format_parameters($queryParameters), $initContent);

        return SlimResponseHelper::response_segment($initContent, $response);
    }


    public function getEpisodeMediaSegment(Request $request, Response $response, array $args): Response
    {
        $encodedInitUrl = $args["encodedInitUrl"];
        $encodedBaseUrl = $args["encodedBaseUrl"];
        $segmentPath = $args["segmentPath"];

        // Because the parameters in the MPD path will be interpreted as query, not part of the URL
        $queryParameters = $request->getQueryParams();

        $originalUrl = base64_decode($encodedBaseUrl, true);
        $originalUrl .= $segmentPath;

        $originalInitUrl = base64_decode($encodedInitUrl, true);

        $id = join("/", [$args["streamingService"], $args["show"], $args["season"], $args["episode"]]);

        $decryptionKeys = $this->manifestController->getDecryptionKeys($id);

        if (!$decryptionKeys) {
            return "Error decryption keys"; // HAHAHA
        }

        $initContent = $this->manifestController->getInitContent($originalInitUrl);

        if (!$initContent) {
            return "Error init content $originalInitUrl"; // TODO: Wow, this sucks, I need to improve error reporting ;-)
        }

        $segmentContent = RequestHelper::get($originalUrl, parameters: $queryParameters); // TODO: Add segments headers

        $decryptedSegmentContent = $this->segmentDecryptor->getDecryptedSegment($initContent, $segmentContent, $decryptionKeys);

        return SlimResponseHelper::response_segment($decryptedSegmentContent, $response);
    }

    //endregion

    public function getEpisodeDownloadInfo(Request $request, array $args): DownloadInfo
    {
        list($episode, $ssResponse) = $this->getEpisodeInfo($request, null, $args, true);
        $downloadInfo = $this->parseEpisodeDownloadInfo($episode, $ssResponse);

        $this->getEpisodeDownloadInfoOptional($episode, $downloadInfo);

        $this->psshRetriever->getPssh($downloadInfo);
        $this->decryptionKeysRetriever->getDecryptionKeys($downloadInfo);

        $id = join("/", [$args["streamingService"], $args["show"], $args["season"], $args["episode"]]);

        $this->manifestController->addDecryptionKeys($id, $downloadInfo->decryptionKeys);

        return $downloadInfo;
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
