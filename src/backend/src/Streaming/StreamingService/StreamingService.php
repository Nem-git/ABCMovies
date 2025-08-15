<?php

declare(strict_types=1);

namespace App\Streaming\StreamingService;

use Psr\Http\Message\ServerRequestInterface as Request;
use App\Streaming\Classes\Show;
use App\Streaming\Classes\Season;
use App\Streaming\Classes\Episode;
use App\Streaming\StreamingService\Helpers\StreamingServiceHelper;
use App\Helpers\SlimRequestParsingHelper;
use App\Config\Constants;
use App\Factory\ObjectFactory;
use App\Streaming\Classes\RecommendationType;

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
    /**
     * Short form description of the streaming service
     */
    public string $shortDescription;
    /**
     * Long form description of the streaming service
     */
    public string $fullDescription;
    /**
     * Card image URL
     */
    public string $imageCard;
    /**
     * Background image URL
     */
    public string $imageBackground;

    //region Parsing

    abstract public function retrieveSearchResults(
        string $query,
        int $amount,
    ): array;
    abstract public function retrieveShowRecommendations(
        Show $show,
        int $amount,
    ): array;
    abstract public function retrieveMediaRecommendations(
        string $type,
        int $amount,
    ): array;
    abstract public function retrieveMovieRecommendations(int $amount): array;
    abstract public function retrieveSerieRecommendations(int $amount): array;
    abstract public function retrieveDocumentaryRecommendations(
        int $amount,
    ): array;
    abstract public function retrieveNextEpisodeRecommendation(
        Show $show,
        Season $season,
        Episode $episode,
    ): Episode;
    abstract public function retrieveRecommendationTypes(): array;
    abstract public function retrieveShow(Show $show): void;
    abstract public function retrieveSeason(Show $show, Season $season): void;
    abstract public function retrieveEpisode(
        Show $show,
        Season $season,
        Episode $episode,
        bool $stream = false,
    ): void;

    //endregion

    //region Tagging

    protected function tagRecommendationType(
        RecommendationType $recommendationType,
    ): void {
        $recommendationType->streamingServiceTag = $this->tag;
    }

    protected function tagShow(Show $show): void
    {
        $show->streamingServiceTag = $this->tag;

        if (isset($show->seasons)) {
            foreach ($show->seasons as $season) {
                $this->tagSeason($show, $season);
            }
        }
    }

    protected function tagSeason(Show $show, Season $season): void
    {
        $season->streamingServiceTag = $this->tag;
        $season->showId = $show->id;

        if (isset($season->episodes)) {
            foreach ($season->episodes as $episode) {
                $this->tagEpisode($show, $season, $episode);
            }
        }
    }

    protected function tagEpisode(
        Show $show,
        Season $season,
        Episode $episode,
    ): void {
        $episode->streamingServiceTag = $this->tag;
        $episode->showId = $show->id;
        $episode->seasonNumber = $season->number;
    }

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
        $searchResults = $this->retrieveSearchResults($query, $amount);

        $searchResults = StreamingServiceHelper::removeDuplicateObjectsInArray(
            $searchResults,
        );

        $searchResults = array_slice($searchResults, 0, $amount);

        foreach ($searchResults as $show) {
            $this->tagShow($show);
        }

        return $searchResults;
    }

    //endregion

    //region Recommendations

    //region Retrieve Recommendation types

    public function getRecommendationTypes(Request $request, array $args): array
    {
        // I should maybe add a ?type=movies kinda parameter
        $recommendationTypes = $this->retrieveRecommendationTypes();

        foreach ($recommendationTypes as $recommendationType) {
            $this->tagRecommendationType($recommendationType);
        }

        return $recommendationTypes;
    }

    //endregion

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

        $showRecommendations = $this->retrieveShowRecommendations(
            $show,
            $amount,
        );

        $showRecommendations = StreamingServiceHelper::removeDuplicateObjectsInArray(
            $showRecommendations,
        );

        $showRecommendations = array_slice($showRecommendations, 0, $amount);

        foreach ($showRecommendations as $show) {
            $this->tagShow($show);
        }

        return $showRecommendations;
    }

    //endregion

    //region Media

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
        $recommendations = $this->retrieveMediaRecommendations($type, $amount);

        $recommendations = StreamingServiceHelper::removeDuplicateObjectsInArray(
            $recommendations,
        );

        $recommendations = array_slice($recommendations, 0, $amount);

        foreach ($recommendations as $show) {
            $this->tagShow($show);
        }

        return $recommendations;
    }

    //endregion

    //region Next Recommendation

    public function getNextEpisodeRecommendation(
        Request $request,
        array $args,
    ): Episode {
        $nextRecommendationCriteria = SlimRequestParsingHelper::parseNextRecommendationCriteria(
            $request,
            $args,
        );

        return $this->executeNextEpisodeRecommendation(
            $nextRecommendationCriteria["showId"],
            $nextRecommendationCriteria["seasonNumber"],
            $nextRecommendationCriteria["episodeNumber"],
        );
    }

    protected function executeNextEpisodeRecommendation(
        string $showId,
        int $seasonNumber,
        int $episodeNumber,
    ): Episode {
        $show = ObjectFactory::createShow();
        $show->id = $showId;

        $season = ObjectFactory::createSeason();
        $season->number = $seasonNumber;

        $episode = ObjectFactory::createEpisode();
        $episode->number = $episodeNumber;

        $nextEpisode = $this->retrieveNextEpisodeRecommendation(
            $show,
            $season,
            $episode,
        );

        $this->tagEpisode($show, $season, $episode);

        return $nextEpisode;
    }

    //endregion

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

        $this->retrieveShow($show);

        $this->tagShow($show);

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
            $seasonInfoCriteria["seasonNumber"],
        );
    }

    public function executeSeasonInfo(string $showId, int $seasonNumber): Season
    {
        $show = ObjectFactory::createShow();
        $show->id = $showId;

        $season = ObjectFactory::createSeason();
        $season->number = $seasonNumber;

        $this->retrieveSeason($show, $season);

        $this->tagSeason($show, $season);

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
            $episodeInfoCriteria["seasonNumber"],
            $episodeInfoCriteria["episodeNumber"],
            false,
        );
    }

    public function executeEpisodeInfo(
        string $showId,
        int $seasonNumber,
        int $episodeNumber,
        bool $stream = false,
    ): Episode {
        $show = ObjectFactory::createShow();
        $show->id = $showId;

        $season = ObjectFactory::createSeason();
        $season->number = $seasonNumber;

        $episode = ObjectFactory::createEpisode();
        $episode->number = $episodeNumber;

        $this->retrieveEpisode($show, $season, $episode, $stream);

        $this->tagEpisode($show, $season, $episode);

        // Only sets the episode url if you don't want to stream right now
        // as it will fuck up the episode->url given in the retrieveEpisode method
        if (!$stream) {
            // TODO: Find the right way to get streaming tech
            $episode->url = StreamingServiceHelper::getStreamUrl(
                $this->tag,
                $show->id,
                $season->number,
                $episode->number,
                Constants::STREAMING_TECH_RANK[0],
            );
        }

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

        return $this->executeEpisodeVideo(
            $episodeVideoCriteria["showId"],
            $episodeVideoCriteria["seasonNumber"],
            $episodeVideoCriteria["episodeNumber"],
            $episodeVideoCriteria["streamingTechnology"],
            $episodeVideoCriteria["extraArgs"],
            $episodeVideoCriteria["queryParams"],
        );
    }

    public function executeEpisodeVideo(
        string $showId,
        int $seasonNumber,
        int $episodeNumber,
        string $streamingTechnology,
        array $queryParams = [],
        array $extraArgs = [],
    ): string {
        $show = ObjectFactory::createShow();
        $show->id = $showId;

        $season = ObjectFactory::createSeason();
        $season->number = $seasonNumber;

        $episode = ObjectFactory::createEpisode();
        $episode->number = $episodeNumber;

        $episode = $this->executeEpisodeInfo(
            $show->id,
            $season->number,
            $episode->number,
            true,
        );

        $streamingTechnology = ObjectFactory::createStreamingTechnology(
            $streamingTechnology,
        );

        // Unsure about removing completely the args when requesting
        // the manifest, as when using filename, there are no extraArgs
        return $streamingTechnology->getVideo(
            $show,
            $season,
            $episode,
            $extraArgs,
            $queryParams,
        );
    }

    //endregion

    //endregion
}
