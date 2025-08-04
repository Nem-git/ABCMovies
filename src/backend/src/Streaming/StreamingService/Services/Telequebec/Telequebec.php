<?php

declare(strict_types=1);

namespace App\Streaming\StreamingService\Services\Telequebec;

use App\Config\Constants;
use App\Streaming\StreamingService\Services\Telequebec\Config;
use App\Streaming\StreamingService\StreamingService;
use App\Streaming\Helpers\RequestHelper;
use App\Streaming\Classes\Show;
use App\Streaming\Classes\Season;
use App\Streaming\Classes\Episode;
use App\Factory\ObjectFactory;

/**
 * Telequebec, a streaming service made by the government of Quebec
 */
final class Telequebec extends StreamingService
{
    public string $name = "Télé-Québec";
    public string $tag = "TLQC";

    //region Parsing

    #[\Override]
    public function retrieveSearchResults(string $query, int $amount): array
    {
        $response = json_decode(
            RequestHelper::get(
                $this->getSearchUrl($query, $amount),
                $this->getSearchParameters($query, $amount),
                $this->getSearchHeaders($query, $amount),
            ),
            true,
        );

        $results = [];

        foreach (
            $response["data"]["blocks"][0]["widgets"][0]["playlist"]["contents"] as $result
        ) {
            $show = ObjectFactory::createShow();

            $show->id = (string) $result["id"];
            $show->title = $result["original_name"]; // For french: ["name"]
            $show->shortDescription = $show->fullDescription =
                $result["short_description"];

            if (isset($result["production_year"])) {
                $show->year = $result["production_year"];
            }

            $show->imageCard = $result["image"]["url"];

            $show->provider = $this->tag;

            $results[] = $show;
        }

        return $results;
    }

    #[\Override]
    public function retrieveShowRecommendations(Show $show, int $amount): array
    {
        $response = json_decode(
            RequestHelper::get(
                $this->getShowRecommendationsUrl($show, $amount),
                $this->getShowRecommendationsParameters($show, $amount),
                $this->getShowRecommendationsHeaders($show, $amount),
            ),
            true,
        );

        $recommendations = [];

        foreach ($response["data"]["screen"]["blocks"] as $category) {
            if ($category["widgets"][0]["playlist"]["type"] === "related") {
                foreach (
                    $category["widgets"][0]["playlist"]["contents"] as $recommendation
                ) {
                    $show = ObjectFactory::createShow();

                    $show->id = (string) $recommendation["id"];
                    $show->title = $recommendation["original_name"]; // For french: ["name"]
                    $show->shortDescription = $show->fullDescription =
                        $recommendation["short_description"];

                    if (isset($recommendation["production_year"])) {
                        $show->year = $recommendation["production_year"];
                    }

                    $show->imageCard = $recommendation["image"]["url"];

                    $show->provider = $this->tag;

                    $results[] = $show;
                }
            }
        }

        return $recommendations;
    }

    #[\Override]
    public function retrieveMovieRecommendations(int $amount): array
    {
        $recommendations = [];

        $response = json_decode(
            RequestHelper::get(
                $this->getMovieRecommendationsUrl($amount),
                $this->getMovieRecommendationsParameters($amount),
                $this->getMovieRecommendationsHeaders($amount),
            ),
            true,
        );

        return $recommendations;
    }

    #[\Override]
    public function retrieveSerieRecommendations(int $amount): array
    {
        $recommendations = [];

        $response = json_decode(
            RequestHelper::get(
                $this->getSerieRecommendationsUrl($amount),
                $this->getSerieRecommendationsParameters($amount),
                $this->getSerieRecommendationsHeaders($amount),
            ),
            true,
        );

        return $recommendations;
    }

    #[\Override]
    public function retrieveDocumentaryRecommendations(int $amount): array
    {
        $recommendations = [];

        $response = json_decode(
            RequestHelper::get(
                $this->getDocumentaryRecommendationsUrl($amount),
                $this->getDocumentaryRecommendationsParameters($amount),
                $this->getDocumentaryRecommendationsHeaders($amount),
            ),
            true,
        );

        return $recommendations;
    }

    // This is totally wrong and I shouldn't do this, but the way I structured my functions made me do it
    #[\Override]
    public function retrieveNextEpisodeRecommendation(
        Show $show,
        Season $season,
        Episode $episode,
    ): Episode {
        // Maybe I should just say that it always returns an episode
        // so I wouldn't have to return an array (yuck) instead of an
        // object

        return ObjectFactory::createEpisode();
    }

    #[\Override]
    public function retrieveShow(Show $show): void
    {
        $response = json_decode(
            RequestHelper::get(
                $this->getShowUrl($show),
                $this->getShowParameters($show),
                $this->getShowHeaders($show),
            ),
            true,
        );

        $response = $response["data"]["asset"];

        $show->id = (string) $response["id"];
        $show->title = $response["original_name"]; // For french: ["name"]
        $show->shortDescription = $response["short_description"];
        $show->fullDescription = $response["long_description"];

        if ($response["production_year"] !== 0) {
            $show->year = $response["production_year"];
        }

        $show->imageCard = $response["images"]["square"]["url"];
        $show->imageBackground = $response["images"]["banner"]["url"]; // Should replace with backdrop but it's rare

        $show->provider = $this->tag;

        if ($response["type"] === "movies") {
            $season = ObjectFactory::createSeason();

            $season->id = $show->id; // Because no season is created by TLQC in movies
            $season->title = Config::DEFAULT_SEASON_TITLE;
            $season->number = Config::DEFAULT_SEASON_NUMBER;
            $season->provider = $this->tag;

            $show->seasons[] = $season;
        }

        if ($response["type"] === "series") {
            foreach ($response["seasons"] as $s) {
                $season = ObjectFactory::createSeason();
                $season->id = (string) $s["id"];
                $season->title = $s["name"];
                $season->number = $s["seasons_number"];
                $season->provider = $this->tag;

                $show->seasons[] = $season;
            }
        }
    }

    private function parseMovieSeason(
        Show $show,
        Season $season,
        array $response,
    ): void {
        $asset = $response["data"]["asset"];

        $season->id = (string) $asset["id"];
        $season->title = Config::DEFAULT_SEASON_TITLE; // TODO: Add method that creates season name
        $season->number = Config::DEFAULT_SEASON_NUMBER; // TODO: WADAFAK ^^
        $season->shortDescription = $asset["short_description"];
        $season->fullDescription = $asset["long_description"];

        $season->provider = $this->tag;

        $episode = ObjectFactory::createEpisode();

        $episode->id = $season->id; // TODO: Doesn't make sense, but idk
        $episode->title = $season->title;
        $episode->number = Config::DEFAULT_EPISODE_NUMBER; // TODO: FIXME
        $episode->shortDescription = $season->shortDescription;
        $episode->fullDescription = $season->fullDescription;
        $episode->imageCard = $asset["images"]["square"]["url"];

        $episode->provider = $this->tag;

        $season->episodes[] = $episode;
    }

    private function parseSerieBlock(
        Show $show,
        Season $season,
        array $block,
    ): void {
        foreach ($block["widgets"][0]["playlist"]["contents"] as $s) {
            if ($season->number === $s["season_number"]) {
                $season->id = (string) $s["id"];
                $season->title = $s["name"];
                $season->number = $s["season_number"];
                $season->shortDescription = $season->fullDescription =
                    $s["original_name"];

                $season->provider = $this->tag;

                break;
            }
        }
    }

    private function retrieveSerieEpisodes(Show $show, Season $season): void
    {
        // parseSerieBlock hasn't seen season block yet
        if (empty($season->id)) {
            return;
        }

        // Request /episodes for the specific season
        $response = json_decode(
            RequestHelper::get(
                $this->getSerieEpisodesUrl($show, $season),
                $this->getSerieEpisodesParameters($show, $season),
                $this->getSerieEpisodesHeaders($show, $season),
            ),
            true,
        );

        foreach ($response["data"] as $e) {
            $episode = ObjectFactory::createEpisode();

            $episode->id = (string) $e["id"];
            $episode->title = $e["original_name"];
            $episode->number = $e["episode_number"];
            $episode->shortDescription = $episode->fullDescription =
                $e["short_description"];
            $episode->imageCard = $e["image"]["url"];

            $episode->provider = $this->tag;

            $season->episodes[] = $episode;
        }
    }

    private function parseSeasonSeries(
        Show $show,
        Season $season,
        array $response,
    ): void {
        foreach ($response["data"]["screen"]["blocks"] as $block) {
            $blockType = $block["widgets"][0]["playlist"]["type"];

            if ($blockType === "seasons") {
                $this->parseSerieBlock($show, $season, $block);
            }

            if ($blockType === "episodes") {
                $this->retrieveSerieEpisodes($show, $season, $block);
            }
        }
    }

    #[\Override]
    public function retrieveSeason(Show $show, Season $season): void
    {
        $response = json_decode(
            RequestHelper::get(
                $this->getSeasonUrl($show, $season),
                $this->getSeasonParameters($show, $season),
                $this->getSeasonHeaders($show, $season),
            ),
            true,
        );

        $type = $response["data"]["asset"]["type"];

        match ($type) {
            "movies" => $this->parseMovieSeason($show, $season, $response),
            "series" => $this->parseSeasonSeries($show, $season, $response),
        };
    }

    #[\Override]
    public function retrieveEpisode(
        Show $show,
        Season $season,
        Episode $episode,
        bool $stream = false,
    ): void {
        $response = json_decode(
            RequestHelper::get(
                $this->getEpisodeUrl($show, $season, $episode),
                $this->getEpisodeParameters($show, $season, $episode),
                $this->getEpisodeHeaders($show, $season, $episode),
            ),
            true,
        );
    }

    //endregion

    //region Specific values

    private function getSearchUrl(string $query, int $amount): string
    {
        return Config::TELEQUEBEC_SEARCH_URL;
    }

    private function getSearchParameters(string $query, int $amount): array
    {
        $parameters = Config::TELEQUEBEC_PARAMETERS_SEARCH;

        $parameters["limit"] = $amount;
        $parameters["q"] = urlencode($query);

        return $parameters;
    }

    private function getSearchHeaders(string $query, int $amount): array
    {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    private function getShowRecommendationsUrl(Show $show, int $amount): string
    {
        return $this->getShowUrl($show);
    }

    private function getShowRecommendationsParameters(
        Show $show,
        int $amount,
    ): array {
        return $this->getShowParameters($show);
    }

    private function getShowRecommendationsHeaders(
        Show $show,
        int $amount,
    ): array {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    private function getMovieRecommendationsUrl(int $amount): string
    {
        return Config::TELEQUEBEC_URL_MOVIES_RECOMMENDATIONS;
    }

    private function getMovieRecommendationsParameters(int $amount): array
    {
        $parameters = Config::TELEQUEBEC_PARAMETERS_MOVIES_RECOMMENDATIONS;
        $parameters["pageSize"] = $amount;
        return $parameters;
    }

    private function getMovieRecommendationsHeaders(int $amount): array
    {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    private function getSerieRecommendationsUrl(int $amount): string
    {
        return Config::TELEQUEBEC_URL_SERIES_RECOMMENDATIONS;
    }

    private function getSerieRecommendationsParameters(int $amount): array
    {
        return $this->getMovieRecommendationsParameters($amount);
    }

    private function getSerieRecommendationsHeaders(int $amount): array
    {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    private function getDocumentaryRecommendationsUrl(int $amount): string
    {
        return Config::TELEQUEBEC_URL_DOCUMENTARIES_RECOMMENDATIONS;
    }

    private function getDocumentaryRecommendationsParameters(int $amount): array
    {
        return $this->getMovieRecommendationsParameters($amount);
    }

    private function getDocumentaryRecommendationsHeaders(int $amount): array
    {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    private function getNextEpisodeRecommendationUrl(
        Show $show,
        Season $season,
        Episode $episode,
    ): string {
        return $this->getSeasonUrl($show, $season);
    }

    private function getNextEpisodeRecommendationParameters(
        Show $show,
        Season $season,
        Episode $episode,
    ): array {
        return $this->getSeasonParameters($show, $season);
    }

    private function getNextEpisodeRecommendationHeaders(
        Show $show,
        Season $season,
        Episode $episode,
    ): array {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    private function getShowUrl(Show $show): string
    {
        return Config::TELEQUEBEC_URL_SHOW_INFO . $show->id;
    }

    private function getShowParameters(Show $show): array
    {
        return Config::TELEQUEBEC_PARAMETERS_SHOW_INFO;
    }

    private function getShowHeaders(Show $show): array
    {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    private function getSeasonUrl(Show $show, Season $season): string
    {
        return $this->getShowUrl($show);
    }

    private function getSeasonParameters(Show $show, Season $season): array
    {
        return $this->getShowParameters($show);
    }

    private function getSeasonHeaders(Show $show, Season $season): array
    {
        return $this->getShowHeaders($show);
    }

    private function getEpisodeUrl(
        Show $show,
        Season $season,
        Episode $episode,
    ): string {
        return $this->getShowUrl($show);
    }

    private function getEpisodeParameters(
        Show $show,
        Season $season,
        Episode $episode,
    ): array {
        return $this->getShowParameters($show);
    }

    private function getEpisodeHeaders(
        Show $show,
        Season $season,
        Episode $episode,
    ): array {
        return $this->getShowHeaders($show);
    }

    //region New functions

    private function getSeasonEpisodesUrl(Show $show, Season $season): string
    {
        return Config::TELEQUEBEC_URL_SEASON_EPISODES_INFO .
            join("/", [$show->id, "season", $season->id, "episodes"]);
    }

    private function getSeasonEpisodesParameters(
        Show $show,
        Season $season,
    ): array {
        return $this->getShowParameters($show);
    }

    private function getSeasonEpisodesHeaders(Show $show, Season $season): array
    {
        return $this->getShowHeaders($show);
    }

    private function getSerieEpisodesUrl(Show $show, Season $season): string
    {
        return $this->getSeasonEpisodesUrl($show, $season);
    }

    private function getSerieEpisodesParameters(
        Show $show,
        Season $season,
    ): array {
        return Config::TELEQUEBEC_PARAMETERS_SEASON_EPISODES_INFO;
    }

    private function getSerieEpisodesHeaders(Show $show, Season $season): array
    {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    //endregion

    //endregion
}
