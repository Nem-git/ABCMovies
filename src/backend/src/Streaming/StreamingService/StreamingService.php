<?php

declare(strict_types=1);

namespace App\Streaming\StreamingService;

use Psr\Http\Message\ServerRequestInterface as Request;
use App\Streaming\Classes\DownloadInfo;
use App\Streaming\Helpers\RequestHelper;
use App\Streaming\Classes\Show;
use App\Streaming\Classes\Season;
use App\Streaming\Classes\Episode;
use App\Streaming\StreamingService\Helpers\StreamingServiceHelper;
use App\Helpers\SlimRequestParsingHelper;
use App\Config\Constants;
use App\Factory\ObjectFactory;

abstract class StreamingService
{
    /**
     * Streaming service's name
     */
    public string $name;
    /**
     * Streaming service's abreviation (EX: DSNP)
     */
    public string $tag;


    //region Parsing

    abstract public function parseSearchResults(array $response): array;
    abstract public function parseShowRecommendationsResults(array $response): array;
    abstract public function parseMoviesRecommendationsResults(array $response): array;
    abstract public function parseSeriesRecommendationsResults(array $response): array;
    abstract public function parseDocumentariesRecommendationsResults(array $response): array;
    abstract public function parseNextRecommendationResult(array $response, string $showId, string $seasonId, string $episodeId): array;
    abstract public function parseShowInfo(Show $show, array $response): void;
    abstract public function parseSeasonInfo(Season $season, array $response): void;
    abstract public function parseEpisodeInfo(Episode $episode, array $response): void;

    //endregion

    //region Get Informations

    abstract public function getEpisodeDownloadInfo(Episode $episode, array $response): DownloadInfo;

    //region Search

    public function getSearchResults(Request $request, array $args): array
    {
        $searchResultsCriteria = SlimRequestParsingHelper::parseSearchCriteria($request, $args);

        return $this->executeSearchResults(
            $searchResultsCriteria["query"],
            $searchResultsCriteria["amount"],
        );
    }

    public function executeSearchResults(string $query, int $amount): array
    {
        $parameters = $this->getSearchParameters($query, $amount);
        $response = RequestHelper::get($this->getSearchUrl($query, $amount), Constants::HTTP_DEFAULT_HEADERS, $parameters);
        $searchResults = $this->parseSearchResults(json_decode($response, true));
        return array_slice($searchResults, 0, $amount);
    }

    //endregion

    //region Recommendations

    //region Show

    public function getShowRecommendations(Request $request, array $args): array
    {
        $showRecommendationsCriteria = SlimRequestParsingHelper::parseShowRecommendationsCriteria($request, $args);

        return $this->executeShowRecommendations(
            $showRecommendationsCriteria["showId"],
            $showRecommendationsCriteria["amount"],
        );
    }

    public function executeShowRecommendations(string $showId, int $amount): array
    {
        $parameters = $this->getShowRecommendationsParameters($showId, $amount);
        $response = RequestHelper::get($this->getShowRecommendationsUrl($showId, $amount), Constants::HTTP_DEFAULT_HEADERS, $parameters);
        $showRecommendations = $this->parseShowRecommendationsResults(json_decode($response, true));
        return array_slice($showRecommendations, 0, $amount);
    }

    //endregion

    //region Movies

    public function getMediaRecommendations(Request $request, array $args): array
    {
        $recommendationsCriteria = SlimRequestParsingHelper::parseRecommendationsCriteria($request, $args);

        return $this->executeMediaRecommendations(
            $recommendationsCriteria["amount"],
            $recommendationsCriteria["type"],
        );
    }

    public function executeMediaRecommendations(int $amount, string $type): array
    {
        $parameters = call_user_func_array([$this, "get".StreamingServiceHelper::getPascalCaseWord($type)."RecommendationsParameters"], [$amount]);
        $recommendationsUrl = call_user_func_array([$this, "get".StreamingServiceHelper::getPascalCaseWord($type)."RecommendationsUrl"], [$amount]);
        $response = RequestHelper::get($recommendationsUrl, Constants::HTTP_DEFAULT_HEADERS, $parameters);
        $recommendations = call_user_func_array([$this, "parse".StreamingServiceHelper::getPascalCaseWord($type)."RecommendationsResults"], [json_decode($response, true)]);
        return array_slice($recommendations, 0, $amount);
    }

    //endregion

    //endregion

    public function getNextRecommendation(Request $request, array $args): array
    {
        $nextRecommendationCriteria = SlimRequestParsingHelper::parseNextRecommendationCriteria($request, $args);

        return $this->executeGetNextRecommendation(
            $nextRecommendationCriteria["showId"],
            $nextRecommendationCriteria["seasonId"],
            $nextRecommendationCriteria["episodeId"],
        );
    }

    public function executeGetNextRecommendation(string $showId, string $seasonId, string $episodeId): array
    {
        $parameters = $this->getNextRecommendationParameters($showId, $seasonId, $episodeId);
        $response = RequestHelper::get($this->getNextRecommendationUrl($showId, $seasonId, $episodeId), Constants::HTTP_DEFAULT_HEADERS, $parameters);
        $nextRecommendation = $this->parseNextRecommendationResult(json_decode($response, true), $showId, $seasonId, $episodeId);
        return $nextRecommendation;
    }

    //endregion

    //region Show

    public function getShowInfo(Request $request, array $args): Show
    {
        $showInfoCriteria = SlimRequestParsingHelper::parseShowInfoCriteria($request, $args);

        return $this->executeShowInfo(
            $showInfoCriteria["showId"],
        );
    }

    public function executeShowInfo(string $showId): Show
    {
        $show = ObjectFactory::createShow();
        $show->id = $showId;

        $response = RequestHelper::get($this->getShowInfoUrl($showId), Constants::HTTP_DEFAULT_HEADERS, $this->getShowInfoParameters($showId));
        $this->parseShowInfo($show, json_decode($response, true));
        return $show;
    }

    //endregion

    //region Season

    public function getSeasonInfo(Request $request, array $args): Season
    {
        $seasonInfoCriteria = SlimRequestParsingHelper::parseSeasonInfoCriteria($request, $args);

        return $this->executeSeasonInfo(
            $seasonInfoCriteria["showId"],
            $seasonInfoCriteria["seasonId"],
        );
    }

    public function executeSeasonInfo(string $showId, string $seasonId): Season
    {
        $season = ObjectFactory::createSeason();
        $season->id = $seasonId;

        $response = RequestHelper::get($this->getSeasonInfoUrl($showId, $seasonId), Constants::HTTP_DEFAULT_HEADERS, $this->getSeasonInfoParameters($showId, $seasonId));
        $this->parseSeasonInfo($season, json_decode($response, true));
        return $season;
    }

    //endregion

    //region Episode

    public function getEpisodeInfo(Request $request, array $args): Episode
    {
        $episodeInfoCriteria = SlimRequestParsingHelper::parseEpisodeInfoCriteria($request, $args);

        return $this->executeEpisodeInfo(
            $episodeInfoCriteria["showId"],
            $episodeInfoCriteria["seasonId"],
            $episodeInfoCriteria["episodeId"],
        );
    }

    public function executeEpisodeInfo(string $showId, string $seasonId, string $episodeId): Episode
    {
        $episode = ObjectFactory::createEpisode();
        $episode->id = $episodeId;
        $episode->url = StreamingServiceHelper::getStreamUrl($this->tag, $showId, $seasonId, $episodeId, Constants::STREAMING_TECH_RANK[0]);

        $response = RequestHelper::get($this->getEpisodeInfoUrl($showId, $seasonId, $episodeId), Constants::HTTP_DEFAULT_HEADERS, $this->getEpisodeInfoParameters($showId, $seasonId, $episodeId));
        $this->parseEpisodeInfo($episode, json_decode($response, true));

        return $episode;
    }

    //endregion

    //region Episode's Video

    /**
     * This gets called whenever the client requests anything related to video,
     * like a DASH manifest or a playlist segment
     */
    public function getEpisodeVideo(Request $request, array $args): string
    {
        $episodeVideoCriteria = SlimRequestParsingHelper::parseEpisodeVideoCriteria($request, $args);

        $streamingTechnology = ObjectFactory::createStreamingTechnology($episodeVideoCriteria["streamingTechnology"]);

        // Unsure about removing completely the args when requesting the manifest, as when using filename, there are no extraArgs
        return $streamingTechnology->getVideo(
            $this,
            $request,
            $episodeVideoCriteria["showId"],
            $episodeVideoCriteria["seasonId"],
            $episodeVideoCriteria["episodeId"],
            isset($args["extraArgs"]) ? explode('/', $args["extraArgs"]) : [],
        );
    }

    //endregion

    //endregion

    //region Abstract methods for URLs and parameters (to be implemented per service)

    abstract public function getSearchUrl(string $query, int $amount): string;
    abstract public function getSearchParameters(string $query, int $amount): array;

    abstract public function getShowRecommendationsUrl(string $showId, int $amount): string;
    abstract public function getShowRecommendationsParameters(string $showId, int $amount): array;

    abstract public function getMoviesRecommendationsUrl(int $amount): string;
    abstract public function getMoviesRecommendationsParameters(int $amount): array;

    abstract public function getSeriesRecommendationsUrl(int $amount): string;
    abstract public function getSeriesRecommendationsParameters(int $amount): array;

    abstract public function getDocumentariesRecommendationsUrl(int $amount): string;
    abstract public function getDocumentariesRecommendationsParameters(int $amount): array;

    abstract public function getNextRecommendationUrl(string $showId, string $seasonId, string $episodeId): string;
    abstract public function getNextRecommendationParameters(string $showId, string $seasonId, string $episodeId): array;

    abstract public function getShowInfoUrl(string $showId): string;
    abstract public function getShowInfoParameters(string $showId): array;

    abstract public function getSeasonInfoUrl(string $showId, string $seasonId): string;
    abstract public function getSeasonInfoParameters(string $showId, string $seasonId): array;

    abstract public function getEpisodeInfoUrl(string $showId, string $seasonId, string $episodeId): string;
    abstract public function getEpisodeInfoParameters(string $showId, string $seasonId, string $episodeId): array;

    //endregion

}
