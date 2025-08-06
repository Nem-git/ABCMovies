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

                    $results[] = $show;
                }
            }
        }

        return $recommendations;
    }

    private function parseMediaRecommendations(
        array $response,
        int $amount,
    ): array {
        $recommendations = [];

        $blocks = $response["data"]["screen"]["blocks"];

        foreach ($blocks as $block) {
            $contents = $block["widgets"][0]["playlist"]["contents"];

            foreach ($contents as $recommendation) {
                $episode = ObjectFactory::createEpisode();

                $episode->id = (string) $recommendation["id"];
                $episode->title = $recommendation["original_name"];
                $episode->shortDescription = $episode->fullDescription =
                    $recommendation["short_description"];
                $episode->imageCard = $recommendation["image"]["url"];

                $recommendations[] = $episode;
            }
        }

        return $recommendations;
    }

    #[\Override]
    public function retrieveMovieRecommendations(int $amount): array
    {
        $response = json_decode(
            RequestHelper::get(
                $this->getMovieRecommendationsUrl($amount),
                $this->getMovieRecommendationsParameters($amount),
                $this->getMovieRecommendationsHeaders($amount),
            ),
            true,
        );

        $recommendations = $this->parseMediaRecommendations($response, $amount);

        return $recommendations;
    }

    #[\Override]
    public function retrieveSerieRecommendations(int $amount): array
    {
        $response = json_decode(
            RequestHelper::get(
                $this->getSerieRecommendationsUrl($amount),
                $this->getSerieRecommendationsParameters($amount),
                $this->getSerieRecommendationsHeaders($amount),
            ),
            true,
        );

        $recommendations = $this->parseMediaRecommendations($response, $amount);

        return $recommendations;
    }

    #[\Override]
    public function retrieveDocumentaryRecommendations(int $amount): array
    {
        $response = json_decode(
            RequestHelper::get(
                $this->getDocumentaryRecommendationsUrl($amount),
                $this->getDocumentaryRecommendationsParameters($amount),
                $this->getDocumentaryRecommendationsHeaders($amount),
            ),
            true,
        );

        $recommendations = $this->parseMediaRecommendations($response, $amount);

        return $recommendations;
    }

    private function parseNextEpisodeInSeasonRecommendation(
        Show $show,
        Season $season,
        Episode $episode,
    ): bool {
        $currentSeason = ObjectFactory::createSeason();

        $currentSeason->number = $season->number;

        $this->retrieveSeason($show, $currentSeason);

        foreach ($currentSeason->episodes as $episodeKey => $e) {
            if ($e->number === $episode->number) {
                if (isset($currentSeason->episodes[$episodeKey + 1])) {
                    $season->id = $currentSeason->id;
                    $season->title = $currentSeason->title;
                    $season->number = $currentSeason->number;
                    $season->shortDescription =
                        $currentSeason->shortDescription;
                    $season->fullDescription = $currentSeason->fullDescription;
                    $season->episodes = $currentSeason->episodes;

                    $nextEpisode = $season->episodes[$episodeKey + 1];

                    $episode->id = $nextEpisode->id;
                    $episode->title = $nextEpisode->title;
                    $episode->number = $nextEpisode->number;
                    $episode->shortDescription = $nextEpisode->shortDescription;
                    $episode->fullDescription = $nextEpisode->fullDescription;
                    $episode->imageCard = $nextEpisode->imageCard;

                    return true;
                }
            }
        }

        return false;
    }

    private function parseFirstEpisodeInNextSeasonRecommendation(
        Show $show,
        Season $season,
        Episode $episode,
    ): bool {
        foreach ($show->seasons as $seasonKey => $s) {
            if ($s->number === $season->number) {
                if (isset($show->seasons[$seasonKey + 1])) {
                    $nextSeason = $show->seasons[$seasonKey + 1];

                    $season->id = $nextSeason->id;
                    $season->title = $nextSeason->title;
                    $season->number = $nextSeason->number;

                    $this->retrieveSeason($show, $season);

                    $firstEpisode = $season->episodes[0];

                    $episode->id = $firstEpisode->id;
                    $episode->title = $firstEpisode->title;
                    $episode->number = $firstEpisode->number;
                    $episode->shortDescription =
                        $firstEpisode->shortDescription;
                    $episode->fullDescription = $firstEpisode->fullDescription;
                    $episode->imageCard = $firstEpisode->imageCard;

                    return true;
                }
            }
        }

        return false;
    }

    private function parseFirstEpisodeInFirstSeasonRecommendation(
        Show $show,
        Season $season,
        Episode $episode,
    ): bool {
        $firstSeason = $show->seasons[0];

        $season->id = $firstSeason->id;
        $season->title = $firstSeason->title;
        $season->number = $firstSeason->number;

        $this->retrieveSeason($show, $season);

        $firstEpisode = $season->episodes[0];

        $episode->id = $firstEpisode->id;
        $episode->title = $firstEpisode->title;
        $episode->number = $firstEpisode->number;
        $episode->shortDescription = $firstEpisode->shortDescription;
        $episode->fullDescription = $firstEpisode->fullDescription;
        $episode->imageCard = $firstEpisode->imageCard;

        return true;
    }

    // This is totally wrong and I shouldn't do this, but the way I structured my functions made me do it
    #[\Override]
    public function retrieveNextEpisodeRecommendation(
        Show $show,
        Season $season,
        Episode $episode,
    ): Episode {
        // I have actually no idea how to comply with this function, because in this case I really need to have access
        // to the show, season and episode. Having only the response will lead to nothing, because I do not have the
        // episode, so I won't know what episode to look for. Maybe I should add some episode info in that function
        // declaration, so that even if the streaming service doesn't implement the logic for you, you can still get the
        // next episode
        // So, this should give back the next episode in the season
        // If that's not possible, give back the first episode of the next season
        // If you're at the end of the show, recommend the first show recommendation

        if ($this->parseNextEpisodeInSeasonRecommendation(
            $show,
            $season,
            $episode,
        )
        ) {
            return $episode;
        }

        $this->retrieveShow($show);

        if ($this->parseFirstEpisodeInNextSeasonRecommendation(
            $show,
            $season,
            $episode,
        )
        ) {
            return $episode;
        }

        if ($this->parseFirstEpisodeInFirstSeasonRecommendation(
            $show,
            $season,
            $episode,
        )
        ) {
            return $episode;
        }

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

        if ($response["type"] === "movies") {
            $season = ObjectFactory::createSeason();

            $season->id = $show->id; // Because no season is created by TLQC in movies
            $season->title = Config::DEFAULT_SEASON_TITLE;
            $season->number = Config::DEFAULT_SEASON_NUMBER;

            $show->seasons[] = $season;
        }

        if ($response["type"] === "series") {
            foreach ($response["seasons"] as $s) {
                $season = ObjectFactory::createSeason();
                $season->id = (string) $s["id"];
                $season->title = $s["name"];
                $season->number = $s["seasons_number"];

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

        $episode = ObjectFactory::createEpisode();

        $episode->id = $season->id; // TODO: Doesn't make sense, but idk
        $episode->title = $asset["original_name"];
        $episode->number = Config::DEFAULT_EPISODE_NUMBER; // TODO: FIXME
        $episode->shortDescription = $season->shortDescription;
        $episode->fullDescription = $season->fullDescription;
        $episode->imageCard = $asset["images"]["square"]["url"];

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
                $this->retrieveSerieEpisodes($show, $season);
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
            default => null,
        };
    }

    private function parseMovieEpisode(Episode $episode, array $asset): void
    {
        $episode->number = Config::DEFAULT_EPISODE_NUMBER;
    }

    private function parseSerieEpisode(Episode $episode, array $asset): void
    {
        $episode->number = $asset["episode_number"];
    }

    private function parseEpisode(Episode $episode, array $response): void
    {
        $asset = $response["data"]["asset"];
        $type = $asset["type"];

        $episode->id = (string) $asset["id"];
        $episode->title = $asset["original_name"];
        $episode->shortDescription = $asset["short_description"];
        $episode->fullDescription = $asset["long_description"];
        $episode->imageCard = $asset["images"]["square"]["url"];

        $episode->containsDrm =
            $asset["video"]["streams"]["encryption"] !== "open";

        match ($type) {
            "movies" => $this->parseMovieEpisode($episode, $asset),
            "episodes" => $this->parseSerieEpisode($episode, $asset),
            default => null,
        };

        // TODO: Add streaming tech scanning
    }

    private function parseStreamingTechnologies(
        Episode $episode,
        array $sources,
    ) {
        // Parses the sources to retrieve all available streaming techs
        foreach ($sources as $source) {
            // String that indicates the streaming tech
            $definingString = isset($source["type"])
                ? strtolower($source["type"])
                : (isset($source["container"])
                    ? strtolower($source["container"])
                    : "");

            if (isset(Constants::WORD_TO_STREAMING_TECH[$definingString])) {
                if (Constants::WORD_TO_STREAMING_TECH[$definingString] === "dash"
                ) {
                    $episode->streamingTechnology = ObjectFactory::createStreamingTechnology(
                        Constants::WORD_TO_STREAMING_TECH[$definingString],
                    );

                    $episode->url = $source["src"];
                    $episode->urlHeaders = [];

                    // There shouldn't be any DRM
                }
            }
        }
    }

    private function getEpisodeStreamInfo(
        Episode $episode,
        array $response,
    ): void {
        $streamId = $response["data"]["asset"]["video"]["streams"]["id"];

        $response = json_decode(
            RequestHelper::get(
                $this->getEpisodeStreamUrl($episode, $streamId),
                $this->getEpisodeStreamParameters($episode, $streamId),
                $this->getEpisodeStreamHeaders($episode, $streamId),
            ),
            true,
        );

        $stream = $response["data"]["stream"];

        $streamUrl = $stream["url"];
        $policyKey = $stream["video_provider_details"]["policy_key"];
        $accountId = $stream["video_provider_details"]["account_id"];

        $response = json_decode(
            RequestHelper::get(
                $this->getEpisodeVideoUrl($accountId, $streamUrl),
                $this->getEpisodeVideoParameters(),
                $this->getEpisodeVideoHeaders($policyKey),
            ),
            true,
        );

        $this->parseStreamingTechnologies($episode, $response["sources"]);
    }

    #[\Override]
    public function retrieveEpisode(
        Show $show,
        Season $season,
        Episode $episode,
        bool $stream = false,
    ): void {
        $this->retrieveSeason($show, $season);

        foreach ($season->episodes as $e) {
            if ($episode->number === $e->number) {
                // Mirror the values from the season's episode to the original episode
                $episode->id = $e->id;
                $episode->title = $e->title;
                $episode->number = $e->number;
                $episode->shortDescription = $e->shortDescription;
                $episode->fullDescription = $e->fullDescription;
                $episode->imageCard = $e->imageCard;

                $response = json_decode(
                    RequestHelper::get(
                        $this->getEpisodeUrl($show, $season, $e),
                        $this->getEpisodeParameters($show, $season, $e),
                        $this->getEpisodeHeaders($show, $season, $e),
                    ),
                    true,
                );

                $this->parseEpisode($episode, $response);

                if ($stream) {
                    $this->getEpisodeStreamInfo($episode, $response);
                }
            }
        }
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
        return Config::TELEQUEBEC_URL_MOVIE_RECOMMENDATIONS;
    }

    private function getMovieRecommendationsParameters(int $amount): array
    {
        $parameters = Config::TELEQUEBEC_PARAMETERS_MOVIE_RECOMMENDATIONS;
        $parameters["pageSize"] = $amount;
        return $parameters;
    }

    private function getMovieRecommendationsHeaders(int $amount): array
    {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    private function getSerieRecommendationsUrl(int $amount): string
    {
        return Config::TELEQUEBEC_URL_SERIE_RECOMMENDATIONS;
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
        return Config::TELEQUEBEC_URL_DOCUMENTARY_RECOMMENDATIONS;
    }

    private function getDocumentaryRecommendationsParameters(int $amount): array
    {
        return $this->getMovieRecommendationsParameters($amount);
    }

    private function getDocumentaryRecommendationsHeaders(int $amount): array
    {
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
        return Config::TELEQUEBEC_URL_SHOW_INFO . $episode->id;
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

    private function getEpisodeStreamUrl(
        Episode $episode,
        string $streamId,
    ): string {
        return Config::TELEQUEBEC_URL_EPISODE_STREAM_INFO .
            $episode->id .
            "/streams/" .
            $streamId;
    }

    private function getEpisodeStreamParameters(
        Episode $episode,
        string $streamId,
    ): array {
        return Config::DEFAULT_PARAMETERS;
    }

    private function getEpisodeStreamHeaders(
        Episode $episode,
        string $streamId,
    ): array {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    private function getEpisodeVideoUrl(
        string $accountId,
        string $streamUrl,
    ): string {
        return Config::TELEQUEBEC_URL_EPISODE_VIDEO .
            $accountId .
            "/videos/" .
            $streamUrl;
    }

    private function getEpisodeVideoParameters(): array
    {
        return [];
    }

    private function getEpisodeVideoHeaders(string $policyKey): array
    {
        $headers = array_merge(
            Constants::HTTP_DEFAULT_HEADERS,
            Config::TELEQUEBEC_HEADERS_EPISODE_VIDEO,
        );

        $headers["Accept"] .= $policyKey;

        return $headers;
    }

    private function getSerieEpisodesUrl(Show $show, Season $season): string
    {
        return Config::TELEQUEBEC_URL_SERIE_EPISODES_INFO .
            join("/", [$show->id, "season", $season->id, "episodes"]);
    }

    private function getSerieEpisodesParameters(
        Show $show,
        Season $season,
    ): array {
        return Config::TELEQUEBEC_PARAMETERS_SERIE_EPISODES_INFO;
    }

    private function getSerieEpisodesHeaders(Show $show, Season $season): array
    {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    //endregion
}
