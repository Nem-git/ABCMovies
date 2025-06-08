<?php

declare(strict_types=1);

require_once __DIR__ . "/Episode.php";
require_once __DIR__ . "/Season.php";
require_once __DIR__ . "/Show.php";

require_once __DIR__ . "/../Constants.php";
require_once __DIR__ . "/../Utilities.php";


use Psr\Http\Message\ServerRequestInterface as Request;
use Psr\Http\Message\ResponseInterface as Response;

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

    abstract protected function parseSearchResults(array $ssResponse): array;
    abstract protected function parseShowInfo(Show $show, array $ssResponse): void;
    abstract protected function parseSeasonInfo(Season $season, array $ssResponse): void;
    abstract protected function parseEpisodeInfo(Episode $episode, array $ssResponse): void;

    public function getSearchResults(Request $request, Response $response, array $args): array
    {
        $query = $args["query"] ?? "*";
        $amount = (int)($request->getQueryParams()["amount"] ?? 20);

        $parameters = $this->getSearchParameters($query, $amount);

        $ssResponse = get_request($this->getSearchUrl(), HTTP_DEFAULT_HEADERS, $parameters);
        return $this->parseSearchResults(json_decode($ssResponse, true));
    }

    public function getShowInfo(Request $request, Response $response, array $args): Show
    {
        $showId = $args["show"] ?? "";
        $show = new Show();
        $show->id = $showId;

        $ssResponse = get_request($this->getShowInfoUrl($showId), HTTP_DEFAULT_HEADERS, $this->getShowInfoParameters());
        $this->parseShowInfo($show, json_decode($ssResponse, true));

        return $show;
    }

    public function getSeasonInfo(Request $request, Response $response, array $args): Season
    {
        $showId = $args["show"] ?? "";
        $seasonId = $args["season"] ?? "";
        $season = new Season();
        $season->id = $seasonId;

        $ssResponse = get_request($this->getSeasonInfoUrl($showId, $seasonId), HTTP_DEFAULT_HEADERS, $this->getSeasonInfoParameters());
        $this->parseSeasonInfo($season, json_decode($ssResponse, true));

        return $season;
    }

    public function getEpisodeInfo(Request $request, Response $response, array $args): Episode
    {
        $showId = $args["show"] ?? "";
        $seasonId = $args["season"] ?? "";
        $episodeId = $args["episode"] ?? "";
        $episode = new Episode();
        $episode->id = $episodeId;

        // TODO: Add verifications to make sure the request has a valid output
        $ssResponse = get_request($this->getEpisodeInfoUrl($showId, $seasonId, $episodeId), HTTP_DEFAULT_HEADERS, $this->getEpisodeInfoParameters());
        $this->parseEpisodeInfo($episode, json_decode($ssResponse, true));

        return $episode;
    }

    // === Abstract methods for URLs and parameters (to be implemented per service) ===

    abstract protected function getSearchUrl(): string;
    abstract protected function getSearchParameters(string $query, int $amount): array;

    abstract protected function getShowInfoUrl(string $showId): string;
    abstract protected function getShowInfoParameters(): array;

    abstract protected function getSeasonInfoUrl(string $showId, string $seasonId): string;
    abstract protected function getSeasonInfoParameters(): array;

    abstract protected function getEpisodeInfoUrl(string $showId, string $seasonId, string $episodeId): string;
    abstract protected function getEpisodeInfoParameters(): array;

}
