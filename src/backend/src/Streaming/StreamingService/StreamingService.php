<?php

declare(strict_types=1);

namespace App\Streaming\StreamingService;

use Psr\Http\Message\ServerRequestInterface as Request;
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
    abstract public function parseShowRecommendationsResults(
        array $response,
    ): array;
    abstract public function parseMoviesRecommendationsResults(
        array $response,
    ): array;
    abstract public function parseSeriesRecommendationsResults(
        array $response,
    ): array;
    abstract public function parseDocumentariesRecommendationsResults(
        array $response,
    ): array;
    abstract public function parseNextRecommendationResult(
        array $response,
        string $showId,
        string $seasonId,
        string $episodeId,
    ): array;
    abstract public function parseShowInfo(Show $show, array $response): void;
    abstract public function parseSeasonInfo(
        Season $season,
        array $response,
    ): void;
    abstract public function parseEpisodeInfo(
        Episode $episode,
        array $ssResponse,
        ?bool $stream,
    ): void;

    //endregion

    //region Search

    public function getSearchResults(Request $request, array $args): array
    {
        $searchResultsCriteria = SlimRequestParsingHelper::parseSearchCriteria(
            $request,
            $args,
        );

        return $this->executeSearchResults(
            $searchResultsCriteria["query"],
            $searchResultsCriteria["amount"],
        );
    }

    public function executeSearchResults(string $query, int $amount): array
    {
        $response = RequestHelper::get(
            $this->getSearchUrl($query, $amount),
            $this->getSearchHeaders($query, $amount),
            $this->getSearchParameters($query, $amount),
        );
        $searchResults = $this->parseSearchResults(
            json_decode($response, true),
        );
        return array_slice($searchResults, 0, $amount);
    }

    //endregion

    //region Recommendations

    //region Show

    public function getShowRecommendations(Request $request, array $args): array
    {
        $showRecommendationsCriteria = SlimRequestParsingHelper::parseShowRecommendationsCriteria(
            $request,
            $args,
        );

        return $this->executeShowRecommendations(
            $showRecommendationsCriteria["showId"],
            $showRecommendationsCriteria["amount"],
        );
    }

    public function executeShowRecommendations(
        string $showId,
        int $amount,
    ): array {
        $show = ObjectFactory::createShow();
        $show->id = $showId;

        $response = RequestHelper::get(
            $this->getShowRecommendationsUrl($show, $amount),
            $this->getShowRecommendationsHeaders($show, $amount),
            $this->getShowRecommendationsParameters($show, $amount),
        );
        $showRecommendations = $this->parseShowRecommendationsResults(
            json_decode($response, true),
        );
        return array_slice($showRecommendations, 0, $amount);
    }

    //endregion

    //region Movies

    public function getMediaRecommendations(
        Request $request,
        array $args,
    ): array {
        $recommendationsCriteria = SlimRequestParsingHelper::parseRecommendationsCriteria(
            $request,
            $args,
        );

        return $this->executeMediaRecommendations(
            $recommendationsCriteria["amount"],
            $recommendationsCriteria["type"],
        );
    }

    // TODO: Fix this abomination, no more calling functions using string, wadafak
    public function executeMediaRecommendations(
        int $amount,
        string $type,
    ): array {
        $parameters = call_user_func_array(
            [
                $this,
                "get" .
                StreamingServiceHelper::getPascalCaseWord($type) .
                "RecommendationsParameters",
            ],
            [$amount],
        );
        $recommendationsUrl = call_user_func_array(
            [
                $this,
                "get" .
                StreamingServiceHelper::getPascalCaseWord($type) .
                "RecommendationsUrl",
            ],
            [$amount],
        );
        $response = RequestHelper::get(
            $recommendationsUrl,
            Constants::HTTP_DEFAULT_HEADERS,
            $parameters,
        );
        $recommendations = call_user_func_array(
            [
                $this,
                "parse" .
                StreamingServiceHelper::getPascalCaseWord($type) .
                "RecommendationsResults",
            ],
            [json_decode($response, true)],
        );
        return array_slice($recommendations, 0, $amount);
    }

    //endregion

    //endregion

    public function getNextRecommendation(Request $request, array $args): array
    {
        $nextRecommendationCriteria = SlimRequestParsingHelper::parseNextRecommendationCriteria(
            $request,
            $args,
        );

        return $this->executeGetNextRecommendation(
            $nextRecommendationCriteria["showId"],
            $nextRecommendationCriteria["seasonId"],
            $nextRecommendationCriteria["episodeId"],
        );
    }

    public function executeGetNextRecommendation(
        string $showId,
        string $seasonId,
        string $episodeId,
    ): array {
        $show = ObjectFactory::createShow();
        $show->id = $showId;

        $season = ObjectFactory::createSeason();
        $season->id = $seasonId;

        $episode = ObjectFactory::createEpisode();
        $episode->id = $episodeId;

        $response = RequestHelper::get(
            $this->getNextRecommendationUrl($show, $season, $episode),
            $this->getNextRecommendationHeaders($show, $season, $episode),
            $this->getNextRecommendationParameters($show, $season, $episode),
        );
        $nextRecommendation = $this->parseNextRecommendationResult(
            json_decode($response, true),
            $showId,
            $seasonId,
            $episodeId,
        );
        return $nextRecommendation;
    }

    //endregion

    //region Show

    public function getShowInfo(Request $request, array $args): Show
    {
        $showInfoCriteria = SlimRequestParsingHelper::parseShowInfoCriteria(
            $request,
            $args,
        );

        return $this->executeShowInfo($showInfoCriteria["showId"]);
    }

    public function executeShowInfo(string $showId): Show
    {
        $show = ObjectFactory::createShow();
        $show->id = $showId;

        $response = RequestHelper::get(
            $this->getShowInfoUrl($show),
            $this->getShowInfoHeaders($show),
            $this->getShowInfoParameters($show),
        );

        $this->parseShowInfo($show, json_decode($response, true));
        return $show;
    }

    //endregion

    //region Season

    public function getSeasonInfo(Request $request, array $args): Season
    {
        $seasonInfoCriteria = SlimRequestParsingHelper::parseSeasonInfoCriteria(
            $request,
            $args,
        );

        return $this->executeSeasonInfo(
            $seasonInfoCriteria["showId"],
            $seasonInfoCriteria["seasonId"],
        );
    }

    public function executeSeasonInfo(string $showId, string $seasonId): Season
    {
        $show = ObjectFactory::createShow();
        $show->id = $showId;

        $season = ObjectFactory::createSeason();
        $season->id = $seasonId;

        $response = RequestHelper::get(
            $this->getSeasonInfoUrl($show, $season),
            $this->getSeasonInfoHeaders($show, $season),
            $this->getSeasonInfoParameters($show, $season),
        );

        $this->parseSeasonInfo($season, json_decode($response, true));
        return $season;
    }

    //endregion

    //region Episode

    public function getEpisodeInfo(Request $request, array $args): Episode
    {
        $episodeInfoCriteria = SlimRequestParsingHelper::parseEpisodeInfoCriteria(
            $request,
            $args,
        );

        return $this->executeEpisodeInfo(
            $episodeInfoCriteria["showId"],
            $episodeInfoCriteria["seasonId"],
            $episodeInfoCriteria["episodeId"],
            false,
        );
    }

    public function executeEpisodeInfo(
        string $showId,
        string $seasonId,
        string $episodeId,
        ?bool $stream,
    ): Episode {
        $show = ObjectFactory::createShow();
        $show->id = $showId;

        $season = ObjectFactory::createSeason();
        $season->id = $seasonId;

        $episode = ObjectFactory::createEpisode();
        $episode->id = $episodeId;

        $episode->url = StreamingServiceHelper::getStreamUrl(
            $this->tag,
            $show->id,
            $season->id,
            $episode->id,
            Constants::STREAMING_TECH_RANK[0],
        );
        // TODO: Find the right way to get streaming tech

        $response = RequestHelper::get(
            $this->getEpisodeInfoUrl($show, $season, $episode),
            $this->getEpisodeInfoHeaders($show, $season, $episode),
            $this->getEpisodeInfoParameters($show, $season, $episode),
        );

        $this->parseEpisodeInfo(
            $episode,
            json_decode($response, true),
            $stream,
        );

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
        $episodeVideoCriteria = SlimRequestParsingHelper::parseEpisodeVideoCriteria(
            $request,
            $args,
        );

        $streamingTechnology = ObjectFactory::createStreamingTechnology(
            $episodeVideoCriteria["streamingTechnology"],
        );

        $episode = $this->executeEpisodeInfo(
            $episodeVideoCriteria["showId"],
            $episodeVideoCriteria["seasonId"],
            $episodeVideoCriteria["episodeId"],
            true,
        );

        // Unsure about removing completely the args when requesting
        // the manifest, as when using filename, there are no extraArgs
        return $streamingTechnology->getVideo(
            $request,
            $episode,
            $episodeVideoCriteria["showId"],
            $episodeVideoCriteria["seasonId"],
            isset($args["extraArgs"]) ? explode("/", $args["extraArgs"]) : [],
        );
    }

    //endregion

    //endregion

    //region Abstract methods for URLs and parameters (to be implemented per service)

    abstract public function getSearchUrl(string $query, int $amount): string;
    abstract public function getSearchParameters(
        string $query,
        int $amount,
    ): array;
    abstract public function getSearchHeaders(
        string $query,
        int $amount,
    ): array;

    abstract public function getShowRecommendationsUrl(
        Show $show,
        int $amount,
    ): string;
    abstract public function getShowRecommendationsParameters(
        Show $show,
        int $amount,
    ): array;
    abstract public function getShowRecommendationsHeaders(
        Show $show,
        int $amount,
    ): array;

    abstract public function getMoviesRecommendationsUrl(int $amount): string;
    abstract public function getMoviesRecommendationsParameters(
        int $amount,
    ): array;
    abstract public function getMoviesRecommendationsHeaders(
        int $amount,
    ): array;

    abstract public function getSeriesRecommendationsUrl(int $amount): string;
    abstract public function getSeriesRecommendationsParameters(
        int $amount,
    ): array;
    abstract public function getSeriesRecommendationsHeaders(
        int $amount,
    ): array;

    abstract public function getDocumentariesRecommendationsUrl(
        int $amount,
    ): string;
    abstract public function getDocumentariesRecommendationsParameters(
        int $amount,
    ): array;
    abstract public function getDocumentariesRecommendationsHeaders(
        int $amount,
    ): array;

    abstract public function getNextRecommendationUrl(
        Show $show,
        Season $season,
        Episode $episode,
    ): string;
    abstract public function getNextRecommendationParameters(
        Show $show,
        Season $season,
        Episode $episode,
    ): array;
    abstract public function getNextRecommendationHeaders(
        Show $show,
        Season $season,
        Episode $episode,
    ): array;

    abstract public function getShowInfoUrl(Show $show): string;
    abstract public function getShowInfoParameters(Show $show): array;
    abstract public function getShowInfoHeaders(Show $show): array;

    abstract public function getSeasonInfoUrl(
        Show $show,
        Season $season,
    ): string;
    abstract public function getSeasonInfoParameters(
        Show $show,
        Season $season,
    ): array;
    abstract public function getSeasonInfoHeaders(
        Show $show,
        Season $season,
    ): array;

    abstract public function getEpisodeInfoUrl(
        Show $show,
        Season $season,
        Episode $episode,
    ): string;
    abstract public function getEpisodeInfoParameters(
        Show $show,
        Season $season,
        Episode $episode,
    ): array;
    abstract public function getEpisodeInfoHeaders(
        Show $show,
        Season $season,
        Episode $episode,
    ): array;

    //endregion
}
