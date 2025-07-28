<?php

declare(strict_types=1);

namespace App\Streaming\StreamingService\Toutv;

use App\Config\Constants;
use App\Streaming\StreamingService\Toutv\Config\ToutvConstants;
use App\Streaming\StreamingService\StreamingService;
use App\Streaming\Helpers\RequestHelper;
use App\Streaming\Classes\Show;
use App\Streaming\Classes\Season;
use App\Streaming\Classes\Episode;
use App\Factory\ObjectFactory;

/**
 * Tou.TV, Radio-Canada's streaming service
 */
final class Toutv extends StreamingService
{
    public string $name = "Tou.TV";
    public string $tag = "TOUTV";

    //region Parsing

    #[\Override]
    public function parseSearchResults(array $ssResponse): array
    {
        $results = [];

        foreach ($ssResponse["results"] as $result) {
            $show = ObjectFactory::createShow();

            $show->id = $result["url"];
            $show->title = $result["title"];
            $show->shortDescription = $show->fullDescription =
                $result["infoTitle"];

            $show->imageCard = $result["images"]["card"]["url"];

            $show->provider = $this->tag;

            if ($result["type"] == "Show") {
                $results[] = $show;
            }
        }

        return $results;
    }

    #[\Override]
    public function parseShowRecommendationsResults(array $response): array
    {
        $recommendations = [];

        foreach ($response["recommendations"]["items"] as $recommendation) {
            $show = ObjectFactory::createShow();

            $show->id = $recommendation["url"];
            $show->title = $recommendation["title"];
            $show->shortDescription = $recommendation["infoTitle"];
            $show->fullDescription = $recommendation["description"];

            $show->imageBackground =
                $recommendation["images"]["background"]["url"];
            $show->imageCard = $recommendation["images"]["card"]["url"];
            $show->provider = $this->tag;

            $recommendations[] = $show;
        }

        return $recommendations;
    }

    #[\Override]
    public function parseMoviesRecommendationsResults(array $response): array
    {
        $recommendations = [];

        foreach (
            $response["content"][0]["items"]["results"] as $recommendation
        ) {
            $show = ObjectFactory::createShow();

            $show->id = $recommendation["url"];
            $show->title = $recommendation["title"];
            $show->shortDescription = $recommendation["infoTitle"];
            $show->fullDescription = $recommendation["description"];

            $show->imageBackground =
                $recommendation["images"]["background"]["url"];
            $show->imageCard = $recommendation["images"]["card"]["url"];
            $show->provider = $this->tag;

            $recommendations[] = $show;
        }

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
        // I have actually no idea how to comply with this function, because in this case I really need to have access
        // to the show, season and episode. Having only the response will lead to nothing, because I do not have the
        // episode, so I won't know what episode to look for. Maybe I should add some episode info in that function
        // declaration, so that even if the streaming service doesn't implement the logic for you, you can still get the
        // next episode
        // So, this should give back the next episode in the season
        // If that's not possible, give back the first episode of the next season
        // If you're at the end of the show, recommend the first show recommendation

        // This loop is for the next episode in the same season
        foreach ($response["content"][0]["lineups"] as $season) {
            foreach ($season["items"] as $episodeKey => $episode) {
                if ($season["seasonNumber"] === (int) $seasonId) {
                    if ($episode["episodeNumber"] === (int) $episodeId) {
                        if (isset($season["items"][$episodeKey + 1])) {
                            $nextEpisode = $season["items"][$episodeKey + 1];
                            $e = ObjectFactory::createEpisode();

                            $e->id = (string) $nextEpisode["idMedia"];
                            $e->title = $nextEpisode["title"];
                            $e->number = $nextEpisode["episodeNumber"];
                            $e->shortDescription = $e->fullDescription =
                                $nextEpisode["description"] ?? "";
                            $e->imageCard =
                                $nextEpisode["images"]["card"]["url"];
                            $e->provider = $this->tag;

                            return (array) $e;
                        }
                    }
                }
            }
        }

        // This loop is for retrieving the next season's first episode
        foreach ($response["content"][0]["lineups"] as $seasonKey => $season) {
            if ($season["seasonNumber"] === (int) $seasonId) {
                if (isset($response["content"][0]["lineups"][$seasonKey + 1])) {
                    $nextSeason =
                        $response["content"][0]["lineups"][$seasonKey + 1];
                    foreach ($nextSeason["items"] as $episode) {
                        $e = ObjectFactory::createEpisode();

                        $e->id = (string) $episode["idMedia"];
                        $e->title = $episode["title"];
                        $e->number = $episode["episodeNumber"];
                        $e->shortDescription = $e->fullDescription =
                            $episode["description"] ?? "";
                        $e->imageCard = $episode["images"]["card"]["url"];
                        $e->provider = $this->tag;

                        // Don't recommend Trailers
                        if ($episode["type"] !== "Trailer") {
                            return (array) $e;
                        }
                    }
                }
            }
        }

        // Returns the first episode of the show
        foreach ($response["content"][0]["lineups"][0]["items"] as $episode) {
            $e = ObjectFactory::createEpisode();

            $e->id = (string) $episode["idMedia"];
            $e->title = $episode["title"];
            $e->number = $episode["episodeNumber"];
            $e->shortDescription = $e->fullDescription =
                $episode["description"] ?? "";
            $e->imageCard = $episode["images"]["card"]["url"];
            $e->provider = $this->tag;

            // Don't recommend Trailers
            if ($episode["type"] !== "Trailer") {
                return (array) $e;
            }
        }

        return [];
    }

    #[\Override]
    public function parseShowInfo(Show $show, array $ssResponse): void
    {
        // Set title, and if it is originally in a language other than french set it to original language
        $show->title = $ssResponse["originalTitle"] ?? $ssResponse["title"];
        $show->shortDescription = $show->fullDescription =
            $ssResponse["description"] ?? "";

        $releaseDate =
            $ssResponse["structuredMetadata"]["datePublished"] ?? null;
        $show->year = (int) explode("-", $releaseDate ? $releaseDate : "")[0]; // The first number being the year

        $show->imageBackground =
            $ssResponse["images"]["background"]["url"] ?? "";
        $show->provider = $this->tag;

        foreach ($ssResponse["content"][0]["lineups"] as $ssResponseSeason) {
            $season = ObjectFactory::createSeason();
            $season->id = (string) $ssResponseSeason["seasonNumber"];
            $season->title = $ssResponseSeason["title"];
            $season->number = $ssResponseSeason["seasonNumber"];
            $season->provider = $this->tag;

            $show->seasons[] = $season;
        }
    }

    #[\Override]
    public function parseSeasonInfo(Season $season, array $ssResponse): void
    {
        foreach ($ssResponse["content"][0]["lineups"] as $ssResponseSeason) {
            // Find the right season that matches the season's ID requested
            if ($ssResponseSeason["seasonNumber"] === (int) $season->id) {
                $season->title = $ssResponseSeason["title"];
                $season->number = (int) $season->id;
                $season->fullDescription = $season->shortDescription =
                    $ssResponse["structuredMetadata"]["abstract"];
                $season->provider = $this->tag;

                // Still not sure if episode should be in Season, but I think it's best to keep it that way for now
                foreach ($ssResponseSeason["items"] as $ssResponseEpisode) {
                    $episode = ObjectFactory::createEpisode();

                    $episode->id = (string) $ssResponseEpisode["idMedia"];
                    $episode->title = $ssResponseEpisode["title"];
                    $episode->number = $ssResponseEpisode["episodeNumber"];
                    $episode->shortDescription = $episode->fullDescription =
                        $ssResponseEpisode["description"] ?? "";
                    $episode->imageCard =
                        $ssResponseEpisode["images"]["card"]["url"];
                    $episode->provider = $this->tag;

                    // Don't add Trailers
                    if ($ssResponseEpisode["type"] !== "Trailer") {
                        $season->episodes[] = $episode;
                    }
                }
                return; // If it enters the if, that's the only time it will
            }
        }
        $season->id = ""; // To clean the season, as the season requested does not exist
    }

    #[\Override]
    public function parseEpisodeInfo(Episode $episode, array $ssResponse): void
    {
        $episode->id = $ssResponse["content.mediaId"];
        $episode->title = $ssResponse["content.title"];
        $episode->number = (int) $ssResponse["content.episode"];
        $episode->provider = $this->tag;
    }

    // TODO: Find the right way to choose the right stream/drm
    // public function getEpisodeStreamingTechnology(Episode $episode)
    // {
    //     $fileParameters = $this->getEpisodeFileParameters($episode);

    //     $ssResponse = RequestHelper::get(
    //         $this->getEpisodeFileUrl($episode),
    //         Constants::HTTP_DEFAULT_HEADERS,
    //         $fileParameters,
    //     );

    //     $this->parseEpisodeFileInfo($episode, json_decode($ssResponse, true));

    // }

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
    ): void {
        // TODO: Actually choose a streaming technology and don't just pick dash with widevine

        // Parses the availableTechs for the available DRM and streaming techs
        foreach ($ssResponse["availableTechs"] as $streamingTechnology) {
            if (array_keys(
                Constants::WORD_TO_STREAMING_TECH,
                $streamingTechnology["name"],
                strict: true,
            )
            ) {
                if ($streamingTechnology["name"] === "dash") {
                    $episode->streamingTechnology = ObjectFactory::createStreamingTechnology(
                        $streamingTechnology["name"],
                    );
                    foreach ($streamingTechnology["drm"] as $drmTechnology) {
                        if ($drmTechnology === "widevine") {
                            $episode->streamingTechnology->drmTechnology = ObjectFactory::createDrmTechnology(
                                $drmTechnology,
                            );
                        }
                    }
                }
            }
        }

        $episode->id = $ssResponse["Metas"]["idMedia"];
        $episode->title = $ssResponse["Metas"]["Title"];
        $episode->number = (int) $ssResponse["Metas"]["SrcEpisode"];

        $episode->fullDescription =
            $ssResponse["Metas"]["Description"] ?:
            $ssResponse["Metas"]["ShortDescription"] ?:
            "";
        $episode->shortDescription = !empty(
            $ssResponse["Metas"]["ShortDescription"]
        )
            ? $ssResponse["Metas"]["ShortDescription"]
            : $episode->fullDescription;

        $episode->containsDrm = (bool) $ssResponse["Metas"]["isDrmActive"];
    }

    public function parseEpisodeDownloadStreamInfo(
        Episode $episode,
        array $ssResponse,
    ): void {
        $episode->url = $ssResponse["url"];
        $episode->urlHeaders = [];

        $episode->streamingTechnology->drmTechnology->licenseHeaders = array_merge(
            $this->getEpisodeDownloadHeaders($episode),
            ToutvConstants::TOUTV_HEADERS_EPISODE_DOWNLOAD_LICENSE_INFO,
        );

        if ($episode->streamingTechnology->drmTechnology->name === "widevine") {
            foreach ($ssResponse["params"] as $parameter) {
                if ($parameter["name"] === "widevineLicenseUrl") {
                    $episode->streamingTechnology->drmTechnology->licenseUrl =
                        $parameter["value"];
                }
                if ($parameter["name"] === "widevineAuthToken") {
                    $episode->streamingTechnology->drmTechnology->licenseHeaders[
                        "x-dt-auth-token"
                    ] = $parameter["value"];
                }
            }
        }
    }

    //endregion

    //region Get TOU.TV values

    #[\Override]
    public function getSearchUrl(string $query, int $amount): string
    {
        return ToutvConstants::TOUTV_URL_SEARCH;
    }

    #[\Override]
    public function getSearchParameters(string $query, int $amount): array
    {
        $parameters = ToutvConstants::TOUTV_PARAMETERS_SEARCH;
        $parameters["pageSize"] = $amount;
        $parameters["term"] = urlencode($query);
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
        $parameters = ToutvConstants::TOUTV_PARAMETERS_SHOW_RECOMMENDATIONS;
        $parameters["pageSize"] = $amount;
        return $parameters;
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
        return ToutvConstants::TOUTV_URL_MOVIES_RECOMMENDATIONS;
    }

    #[\Override]
    public function getMoviesRecommendationsParameters(int $amount): array
    {
        $parameters = ToutvConstants::TOUTV_PARAMETERS_MOVIES_RECOMMENDATIONS;
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
        return ToutvConstants::TOUTV_URL_SERIES_RECOMMENDATIONS;
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
        return ToutvConstants::TOUTV_URL_DOCUMENTARIES_RECOMMENDATIONS;
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
        string $showId,
        string $seasonId,
        string $episodeId,
    ): string {
        return $this->getSeasonInfoUrl($showId, $seasonId);
    }

    #[\Override]
    public function getNextRecommendationParameters(
        string $showId,
        string $seasonId,
        string $episodeId,
    ): array {
        return $this->getSeasonInfoParameters($showId, $seasonId);
    }

    #[\Override]
    public function getNextRecommendationHeaders(
        string $showId,
        string $seasonId,
        string $episodeId,
    ): array {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    #[\Override]
    public function getShowInfoUrl(Show $show): string
    {
        return ToutvConstants::TOUTV_URL_SHOW_INFO . $show->id;
    }

    #[\Override]
    public function getShowInfoParameters(Show $show): array
    {
        return ToutvConstants::TOUTV_PARAMETERS_SHOW_INFO;
    }

    #[\Override]
    public function getShowInfoHeaders(Show $show): array
    {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    #[\Override]
    public function getSeasonInfoUrl(string $showId, string $seasonId): string
    {
        return ToutvConstants::TOUTV_URL_SEASON_INFO .
            $showId .
            "/s" .
            $seasonId;
    }

    #[\Override]
    public function getSeasonInfoParameters(
        string $showId,
        string $seasonId,
    ): array {
        return ToutvConstants::TOUTV_PARAMETERS_SEASON_INFO;
    }

    #[\Override]
    public function getSeasonInfoHeaders(
        string $showId,
        string $seasonId,
    ): array {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    #[\Override]
    public function getEpisodeInfoUrl(
        string $showId,
        string $seasonId,
        string $episodeId,
    ): string {
        // THis %02d adds a trailing 0 in front of the number as a string, like 01 instead of 1
        return ToutvConstants::TOUTV_URL_EPISODE_INFO .
            $showId .
            "/s" .
            sprintf("%02d", $seasonId) .
            "e" .
            sprintf("%02d", $episodeId);
    }

    #[\Override]
    public function getEpisodeInfoParameters(
        string $showId,
        string $seasonId,
        string $episodeId,
    ): array {
        return ToutvConstants::TOUTV_PARAMETERS_EPISODE_INFO;
    }

    #[\Override]
    public function getEpisodeInfoHeaders(
        string $showId,
        string $seasonId,
        string $episodeId,
    ): array {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    //region New functions

    private function getEpisodeFileUrl(Episode $episode): string
    {
        return ToutvConstants::TOUTV_URL_EPISODE_FILE_INFO;
    }

    private function getEpisodeFileParameters(Episode $episode): array
    {
        $parameters = ToutvConstants::TOUTV_PARAMETERS_EPISODE_FILE_INFO;
        $parameters["idMedia"] = $episode->id;
        return $parameters;
    }

    public function getEpisodeFileHeaders(Episode $episode): array
    {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    private function getEpisodeDownloadUrl(Episode $episode): string
    {
        return ToutvConstants::TOUTV_URL_EPISODE_DOWNLOAD_INFO;
    }

    private function getEpisodeDownloadParameters(Episode $episode): array
    {
        $parameters = ToutvConstants::TOUTV_PARAMETERS_EPISODE_DOWNLOAD_INFO;
        $parameters["idMedia"] = $episode->id;
        return $parameters;
    }

    private function getEpisodeDownloadHeaders(Episode $episode): array
    {
        $headers = ToutvConstants::TOUTV_HEADERS_EPISODE_DOWNLOAD_INFO;
        $headers["Authorization"] = "";
        $headers["x-claims-token"] = "";
        $headers = array_merge(Constants::HTTP_DEFAULT_HEADERS, $headers);
        return $headers;
    }

    //endregion

    //endregion
}
