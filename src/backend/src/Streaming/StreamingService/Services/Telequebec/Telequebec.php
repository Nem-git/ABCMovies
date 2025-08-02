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
    public function parseSearchResults(array $ssResponse): array
    {
        $results = [];

        foreach (
            $ssResponse["data"]["blocks"][0]["widgets"][0]["playlist"]["contents"] as $result
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
    public function parseShowRecommendationsResults(array $response): array
    {
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
    public function parseMoviesRecommendationsResults(array $response): array
    {
        $recommendations = [];

        return $recommendations;
    }

    #[\Override]
    public function parseSeriesRecommendationsResults(array $response): array
    {
        return $this->parseMoviesRecommendationsResults($response);
    }

    #[\Override]
    public function parseDocumentariesRecommendationsResults(
        array $response,
    ): array {
        return $this->parseMoviesRecommendationsResults($response);
    }

    // This is totally wrong and I shouldn't do this, but the way I structured my functions made me do it
    #[\Override]
    public function parseNextRecommendationResult(
        array $response,
        string $showId,
        string $seasonId,
        string $episodeId,
    ): array {
        // Maybe I should just say that it always returns an episode
        // so I wouldn't have to return an array (yuck) instead of an
        // object

        return [];
    }

    #[\Override]
    public function parseShowInfo(Show $show, array $ssResponse): void
    {
        $response = $ssResponse["data"]["asset"];

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

            $season->id = (string) Config::DEFAULT_SEASON_NUMBER;
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

    #[\Override]
    public function parseSeasonInfo(Season $season, array $ssResponse): void
    {
        $type = $ssResponse["data"]["asset"]["type"];

        if ($type === "movies") {
            $response = $ssResponse["data"]["asset"];

            if ($season->id === (string) Config::DEFAULT_SEASON_NUMBER) {
                $season->title = Config::DEFAULT_SEASON_TITLE; // TODO: Add method that creates season name
                $season->number = Config::DEFAULT_SEASON_NUMBER; // TODO: WADAFAK ^^
                $season->shortDescription = $response["short_description"];
                $season->fullDescription = $response["long_description"];

                $season->provider = $this->tag;

                foreach ($response["streams"] as $e) {
                    $episode = ObjectFactory::createEpisode();

                    $episode->id = $e["id"];
                    $episode->title = $season->title;
                    $episode->number = 1; // TODO: FIXME
                    $episode->shortDescription = $season->shortDescription;
                    $episode->fullDescription = $season->fullDescription;
                    $episode->imageCard = $response["images"]["square"]["url"];
                    $episode->provider = $this->tag;

                    $season->episodes[] = $episode;
                }
            } else {
                $season->id = ""; // TOOD: FIXME
            }
        }

        if ($type === "series") {
            foreach ($ssResponse["data"]["screen"]["blocks"] as $block) {
                if ($block["widgets"][0]["playlist"]["type"] === "seasons") {
                    foreach (
                        $block["widgets"][0]["playlist"]["contents"] as $s
                    ) {
                        if ($season->id === (string) $s["id"]) {
                            $season->title = $s["name"];
                            $season->number = $s["season_number"];
                            $season->shortDescription = $season->fullDescription =
                                $s["original_name"];

                            $season->provider = $this->tag;

                            break;
                        }
                    }
                }

                if (
                    $season->id === $block["widgets"][0]["playlist"]["contents"][0]["season_number"]
                ) {
                    if (
                        $block["widgets"][0]["playlist"]["type"] === "episodes"
                    ) {
                        foreach (
                            $block["widgets"][0]["playlist"]["contents"] as $e
                        ) {
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
                } else {
                    $showId = (string) $ssResponse["data"]["asset"]["id"];
                    $seasonSlug = $s["slug"];
                    $url = $this->getSeasonEpisodesInfoUrl(
                        $showId,
                        $seasonSlug,
                    );
                    $ssResponse = json_decode(
                        RequestHelper::get(
                            $url,
                            Constants::HTTP_DEFAULT_HEADERS,
                            Config::TELEQUEBEC_PARAMETERS_SEASON_EPISODES_INFO,
                        ),
                        true,
                    );

                    foreach ($ssResponse["data"] as $e) {
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
                    break;
                }
            }
        }
    }

    #[\Override]
    public function parseEpisodeInfo(
        Episode $episode,
        array $ssResponse,
    ): void {}

    #[\Override]
    public function getEpisodeStreamInfo(Episode $episode): void
    {
        $ssResponse = RequestHelper::get(
            $this->getEpisodeFileUrl($episode),
            $this->getEpisodeFileHeaders($episode),
            $this->getEpisodeFileParameters($episode),
        );

        $this->parseEpisodeFileInfo($episode, json_decode($ssResponse, true));

        $ssResponse = RequestHelper::get(
            $this->getEpisodeDownloadUrl($episode),
            $this->getEpisodeDownloadHeaders($episode),
            $this->getEpisodeDownloadParameters($episode),
        );

        $this->parseEpisodeDownloadStreamInfo(
            $episode,
            json_decode($ssResponse, true),
        );
    }

    private function parseEpisodeFileInfo(
        Episode $episode,
        array $ssResponse,
    ): void {}

    public function parseEpisodeDownloadStreamInfo(
        Episode $episode,
        array $ssResponse,
    ): void {}

    //endregion

    //region Specific values

    #[\Override]
    public function getSearchUrl(string $query, int $amount): string
    {
        return Config::TELEQUEBEC_SEARCH_URL;
    }

    #[\Override]
    public function getSearchParameters(string $query, int $amount): array
    {
        $parameters = Config::TELEQUEBEC_PARAMETERS_SEARCH;

        $parameters["limit"] = $amount;
        $parameters["q"] = urlencode($query);

        return $parameters;
    }

    #[\Override]
    public function getSearchHeaders(string $query, int $amount): array
    {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    #[\Override]
    public function getShowRecommendationsUrl(Show $show, int $amount): string
    {
        return $this->getShowInfoUrl($show);
    }

    #[\Override]
    public function getShowRecommendationsParameters(
        Show $show,
        int $amount,
    ): array {
        return $this->getShowInfoParameters($show);
    }

    #[\Override]
    public function getShowRecommendationsHeaders(
        Show $show,
        int $amount,
    ): array {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    #[\Override]
    public function getMoviesRecommendationsUrl(int $amount): string
    {
        return Config::TELEQUEBEC_URL_MOVIES_RECOMMENDATIONS;
    }

    #[\Override]
    public function getMoviesRecommendationsParameters(int $amount): array
    {
        $parameters = Config::TELEQUEBEC_PARAMETERS_MOVIES_RECOMMENDATIONS;
        $parameters["pageSize"] = $amount;
        return $parameters;
    }

    #[\Override]
    public function getMoviesRecommendationsHeaders(int $amount): array
    {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    #[\Override]
    public function getSeriesRecommendationsUrl(int $amount): string
    {
        return Config::TELEQUEBEC_URL_SERIES_RECOMMENDATIONS;
    }

    #[\Override]
    public function getSeriesRecommendationsParameters(int $amount): array
    {
        return $this->getMoviesRecommendationsParameters($amount);
    }

    #[\Override]
    public function getSeriesRecommendationsHeaders(int $amount): array
    {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    #[\Override]
    public function getDocumentariesRecommendationsUrl(int $amount): string
    {
        return Config::TELEQUEBEC_URL_DOCUMENTARIES_RECOMMENDATIONS;
    }

    #[\Override]
    public function getDocumentariesRecommendationsParameters(
        int $amount,
    ): array {
        return $this->getMoviesRecommendationsParameters($amount);
    }

    #[\Override]
    public function getDocumentariesRecommendationsHeaders(int $amount): array
    {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    #[\Override]
    public function getNextRecommendationUrl(
        Show $show,
        Season $season,
        Episode $episode,
    ): string {
        return $this->getSeasonInfoUrl($show, $season);
    }

    #[\Override]
    public function getNextRecommendationParameters(
        Show $show,
        Season $season,
        Episode $episode,
    ): array {
        return $this->getSeasonInfoParameters($show, $season);
    }

    #[\Override]
    public function getNextRecommendationHeaders(
        Show $show,
        Season $season,
        Episode $episode,
    ): array {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    #[\Override]
    public function getShowInfoUrl(Show $show): string
    {
        return Config::TELEQUEBEC_URL_SHOW_INFO . $show->id;
    }

    #[\Override]
    public function getShowInfoParameters(Show $show): array
    {
        return Config::TELEQUEBEC_PARAMETERS_SHOW_INFO;
    }

    #[\Override]
    public function getShowInfoHeaders(Show $show): array
    {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    #[\Override]
    public function getSeasonInfoUrl(Show $show, Season $season): string
    {
        return $this->getShowInfoUrl($show);
    }

    #[\Override]
    public function getSeasonInfoParameters(Show $show, Season $season): array
    {
        return $this->getShowInfoParameters($show);
    }

    #[\Override]
    public function getSeasonInfoHeaders(Show $show, Season $season): array
    {
        return $this->getShowInfoHeaders($show);
    }

    public function getSeasonEpisodesInfoUrl(
        string $showId,
        string $seasonSlug,
    ): string {
        return Config::TELEQUEBEC_URL_SEASON_EPISODES_INFO .
            join("/", [$showId, "season", $seasonSlug, "episodes"]);
    }

    public function getSeasonEpisodesInfoParameters(
        Show $show,
        Season $season,
    ): array {
        return $this->getShowInfoParameters($show);
    }

    public function getSeasonEpisodesInfoHeaders(
        Show $show,
        Season $season,
    ): array {
        return $this->getShowInfoHeaders($show);
    }

    #[\Override]
    public function getEpisodeInfoUrl(
        Show $show,
        Season $season,
        Episode $episode,
    ): string {
        // THis %02d adds a trailing 0 in front of the number as a string, like 01 instead of 1
        return Config::TELEQUEBEC_URL_EPISODE_INFO .
            $show->id .
            "/s" .
            sprintf("%02d", $season->id) .
            "e" .
            sprintf("%02d", $episode->id);
    }

    #[\Override]
    public function getEpisodeInfoParameters(
        Show $show,
        Season $season,
        Episode $episode,
    ): array {
        return Config::TELEQUEBEC_PARAMETERS_EPISODE_INFO;
    }

    #[\Override]
    public function getEpisodeInfoHeaders(
        Show $show,
        Season $season,
        Episode $episode,
    ): array {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    //region New functions

    private function getEpisodeFileUrl(Episode $episode): string
    {
        return Config::TELEQUEBEC_URL_EPISODE_FILE_INFO;
    }

    private function getEpisodeFileParameters(Episode $episode): array
    {
        $parameters = Config::TELEQUEBEC_PARAMETERS_EPISODE_FILE_INFO;
        $parameters["idMedia"] = $episode->id;
        return $parameters;
    }

    public function getEpisodeFileHeaders(Episode $episode): array
    {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    private function getEpisodeDownloadUrl(Episode $episode): string
    {
        return Config::TELEQUEBEC_URL_EPISODE_DOWNLOAD_INFO;
    }

    private function getEpisodeDownloadParameters(Episode $episode): array
    {
        $parameters = Config::TELEQUEBEC_PARAMETERS_EPISODE_DOWNLOAD_INFO;
        $parameters["idMedia"] = $episode->id;
        return $parameters;
    }

    private function getEpisodeDownloadHeaders(Episode $episode): array
    {
        $headers = Config::TELEQUEBEC_HEADERS_EPISODE_DOWNLOAD_INFO;
        $headers["Authorization"] = "";
        $headers["x-claims-token"] = "";
        $headers = array_merge(Constants::HTTP_DEFAULT_HEADERS, $headers);
        return $headers;
    }

    //endregion

    //endregion
}
