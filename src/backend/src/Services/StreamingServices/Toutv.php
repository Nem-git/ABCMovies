<?php

declare(strict_types=1);

namespace App\Services\StreamingServices;

use App\Helpers\RequestHelper;
use App\Models\DownloadInfo;
use App\Services\StreamingService;
use App\Models\Show;
use App\Models\Season;
use App\Models\Episode;
use App\Models\ObjectFactory;

/**
 * Tou.TV, Radio-Canada's streaming service
 */
class Toutv extends StreamingService
{
    protected string $name = "Tou.TV";
    protected string $tag = "TOUTV";

    //region Parsing

    protected function parseSearchResults(array $ssResponse): array
    {
        $results = [];

        foreach ($ssResponse["results"] as $result) {

            $show = ObjectFactory::createShow();

            $show->id = $result["url"];
            $show->title = $result["title"];
            $show->shortDescription = $show->fullDescription = $result["infoTitle"];

            $show->imageCard = $result["images"]["card"]["url"];

            $show->provider = $this->tag;

            if ($result["type"] == "Show") {
                $results[] = $show;
            }
        }

        return $results;
    }

    protected function parseShowRecommendationsResults(array $response): array
    {
        $recommendations = [];

        foreach ($response["recommendations"]["items"] as $recommendation) {
            $show = ObjectFactory::createShow();

            $show->id = $recommendation["url"];
            $show->title = $recommendation["title"];
            $show->shortDescription = $recommendation["infoTitle"];
            $show->fullDescription = $recommendation["description"];

            $show->imageBackground = $recommendation["images"]["background"]["url"];
            $show->imageCard = $recommendation["images"]["card"]["url"];
            $show->provider = $this->tag;

            $recommendations[] = $show;
        }

        return $recommendations;
    }

    protected function parseMoviesRecommendationsResults(array $response): array
    {
        $recommendations = [];

        foreach ($response["content"][0]["items"]["results"] as $recommendation) {
            $show = ObjectFactory::createShow();

            $show->id = $recommendation["url"];
            $show->title = $recommendation["title"];
            $show->shortDescription = $recommendation["infoTitle"];
            $show->fullDescription = $recommendation["description"];

            $show->imageBackground = $recommendation["images"]["background"]["url"];
            $show->imageCard = $recommendation["images"]["card"]["url"];
            $show->provider = $this->tag;

            $recommendations[] = $show;
        }

        return $recommendations;
    }

    protected function parseSeriesRecommendationsResults(array $response): array
    {
        return $this->parseMoviesRecommendationsResults($response);
    }

    protected function parseDocumentariesRecommendationsResults(array $response): array
    {
        return $this->parseMoviesRecommendationsResults($response);
    }

    // This is totally wrong and I shouldn't do this, but the way I structured my functions made me do it
    protected function parseNextRecommendationResult(array $response, string $showId, string $seasonId, string $episodeId): array
    {
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
                if ($season["seasonNumber"] === (int)$seasonId) {
                    if ($episode["episodeNumber"] === (int)$episodeId) {
                        if (isset($season["items"][$episodeKey + 1])) {
                            $nextEpisode = $season["items"][$episodeKey + 1];
                            $e = ObjectFactory::createEpisode();

                            $e->id = (string)$nextEpisode["idMedia"];
                            $e->title = $nextEpisode["title"];
                            $e->number = $nextEpisode["episodeNumber"];
                            $e->shortDescription = $e->fullDescription = $nextEpisode["description"] ?? "";
                            $e->imageCard = $nextEpisode["images"]["card"]["url"];
                            $e->provider = $this->tag;

                            return (array)$e;
                        }
                    }
                }

            }
        }

        // This loop is for retrieving the next season's first episode
        foreach ($response["content"][0]["lineups"] as $seasonKey => $season) {
            if ($season["seasonNumber"] === (int)$seasonId) {
                if (isset($response["content"][0]["lineups"][$seasonKey + 1])) {
                    $nextSeason = $response["content"][0]["lineups"][$seasonKey + 1];
                    foreach ($nextSeason["items"] as $episode) {
                        $e = ObjectFactory::createEpisode();

                        $e->id = (string)$episode["idMedia"];
                        $e->title = $episode["title"];
                        $e->number = $episode["episodeNumber"];
                        $e->shortDescription = $e->fullDescription = $episode["description"] ?? "";
                        $e->imageCard = $episode["images"]["card"]["url"];
                        $e->provider = $this->tag;

                        // Don't add Trailers
                        if ($episode["type"] !== "Trailer") {
                            return (array)$e;
                        }
                    }
                }
            }
        }

        // This is for retrieving the first show's recommendation
        return $this->executeShowRecommendations($showId, 1);
    }

    protected function parseShowInfo(Show $show, array $ssResponse): void
    {
        // Set title, and if it is originally in a language other than french set it to original language
        $show->title = $ssResponse["originalTitle"] ?? $ssResponse["title"];
        $show->shortDescription = $show->fullDescription = $ssResponse["description"] ?? "";

        $releaseDate = $ssResponse["structuredMetadata"]["datePublished"] ?? null;
        $show->year = (int)explode("-", $releaseDate ? $releaseDate : "")[0]; // The first number being the year

        $show->imageBackground = $ssResponse["images"]["background"]["url"];
        $show->provider = $this->tag;

        foreach ($ssResponse["content"][0]["lineups"] as $ssResponseSeason) {
            $season = ObjectFactory::createSeason();
            $season->id = (string)$ssResponseSeason["seasonNumber"];
            $season->title = $ssResponseSeason["title"];
            $season->number = $ssResponseSeason["seasonNumber"];
            $season->provider = $this->tag;

            $show->seasons[] = $season;
        }
    }

    protected function parseSeasonInfo(Season $season, array $ssResponse): void
    {
        foreach ($ssResponse["content"][0]["lineups"] as $ssResponseSeason) {

            // Find the right season that matches the season's ID requested
            if ($ssResponseSeason["seasonNumber"] === (int)$season->id) {

                $season->title = $ssResponseSeason["title"];
                $season->number = (int)$season->id;
                $season->fullDescription = $season->shortDescription = $ssResponse["structuredMetadata"]["abstract"];
                $season->provider = $this->tag;

                // Still not sure if episode should be in Season, but I think it's best to keep it that way for now
                foreach ($ssResponseSeason["items"] as $ssResponseEpisode) {
                    $episode = ObjectFactory::createEpisode();

                    $episode->id = (string)$ssResponseEpisode["idMedia"];
                    $episode->title = $ssResponseEpisode["title"];
                    $episode->number = $ssResponseEpisode["episodeNumber"];
                    $episode->shortDescription = $episode->fullDescription = $ssResponseEpisode["description"] ?? "";
                    $episode->imageCard = $ssResponseEpisode["images"]["card"]["url"];
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

    protected function parseEpisodeInfo(Episode $episode, array $ssResponse): void
    {
        $episode->id = $ssResponse["idFichierToutv"];
        $episode->title = $ssResponse["emission"];
        $episode->number = (int)$ssResponse["episode"];
        $episode->provider = $this->tag;
    }

    protected function parseEpisodeDownloadInfo(Episode $episode, array $ssResponse): DownloadInfo
    {
        $downloadInfo = ObjectFactory::createDownloadInfo();

        $fileParameters = $this->getEpisodeFileParameters($episode->id);
        $ssResponse = RequestHelper::get($this->getEpisodeFileUrl($episode->id), HTTP_DEFAULT_HEADERS, $fileParameters);
        $this->parseEpisodeFileInfo($episode, json_decode($ssResponse, true));

        $headers = $this->getEpisodeDownloadHeaders();
        $downloadParameters = $this->getEpisodeDownloadParameters($episode->id);
        $ssResponse = RequestHelper::get($this->getEpisodeDownloadUrl(), $headers, $downloadParameters);
        $this->parseEpisodeDownloadStreamInfo($episode, json_decode($ssResponse, true), $downloadInfo);

        return $downloadInfo;
    }

    private function parseEpisodeFileInfo(Episode $episode, array $ssResponse): void
    {
        $episode->id = $ssResponse["Metas"]["idMedia"];
        $episode->title = $ssResponse["Metas"]["Title"];
        $episode->number = (int)$ssResponse["Metas"]["SrcEpisode"];

        $episode->fullDescription = $ssResponse["Metas"]["Description"];
        $episode->shortDescription = !empty($ssResponse["Metas"]["ShortDescription"]) ? $ssResponse["Metas"]["ShortDescription"] : $episode->fullDescription;
    }

    public function parseEpisodeDownloadStreamInfo(Episode $episode, array $ssResponse, DownloadInfo $downloadInfo): void
    {
        $downloadInfo->licenseHeaders = array_merge(HTTP_DEFAULT_HEADERS, TOUTV_HEADERS_EPISODE_DOWNLOAD_LICENSE_INFO);

        $downloadInfo->mpdUrl = $ssResponse["url"];

        foreach ($ssResponse["params"] as $parameter) {

            if ($parameter["name"] === "widevineLicenseUrl") {
                $downloadInfo->licenseUrl = $parameter["value"];
            }
            if ($parameter["name"] === "widevineAuthToken") {
                $downloadInfo->licenseHeaders["x-dt-auth-token"] = $parameter["value"];
            }
        }
    }

    //endregion

    //region Get TOU.TV values

    protected function getSearchUrl(string $query, int $amount): string
    {
        return TOUTV_URL_SEARCH;
    }

    protected function getSearchParameters(string $query, int $amount): array
    {
        $parameters = TOUTV_PARAMETERS_SEARCH;
        $parameters["pageSize"] = $amount;
        $parameters["term"] = urlencode($query);
        return $parameters;
    }

    protected function getShowRecommendationsUrl(string $showId, int $amount): string
    {
        return $this->getShowInfoUrl($showId);
    }

    protected function getShowRecommendationsParameters(string $showId, int $amount): array
    {
        $parameters = TOUTV_PARAMETERS_SHOW_RECOMMENDATIONS;
        $parameters["pageSize"] = $amount;
        return $parameters;
    }

    protected function getMoviesRecommendationsUrl(int $amount): string
    {
        return TOUTV_URL_MOVIES_RECOMMENDATIONS;
    }

    protected function getMoviesRecommendationsParameters(int $amount): array
    {
        $parameters = TOUTV_PARAMETERS_MOVIES_RECOMMENDATIONS;
        $parameters["pageSize"] = $amount;
        return $parameters;
    }

    protected function getSeriesRecommendationsUrl(int $amount): string
    {
        return TOUTV_URL_SERIES_RECOMMENDATIONS;
    }

    protected function getSeriesRecommendationsParameters(int $amount): array
    {
        return $this->getMoviesRecommendationsParameters($amount);
    }

    protected function getDocumentariesRecommendationsUrl(int $amount): string
    {
        return TOUTV_URL_DOCUMENTARIES_RECOMMENDATIONS;
    }

    protected function getDocumentariesRecommendationsParameters(int $amount): array
    {
        return $this->getMoviesRecommendationsParameters($amount);
    }

    protected function getNextRecommendationUrl(string $showId, string $seasonId, string $episodeId): string
    {
        return $this->getSeasonInfoUrl($showId, $seasonId);
    }

    protected function getNextRecommendationParameters(string $showId, string $seasonId, string $episodeId): array
    {
        return $this->getSeasonInfoParameters($showId, $seasonId);
    }

    protected function getShowInfoUrl(string $showId): string
    {
        return TOUTV_URL_SHOW_INFO . $showId;
    }

    protected function getShowInfoParameters(string $showId): array
    {
        return TOUTV_PARAMETERS_SHOW_INFO;
    }

    protected function getSeasonInfoUrl(string $showId, string $seasonId): string
    {
        return TOUTV_URL_SEASON_INFO . $showId . "/s" . $seasonId;
    }

    protected function getSeasonInfoParameters(string $showId, string $seasonId): array
    {
        return TOUTV_PARAMETERS_SEASON_INFO;
    }

    protected function getEpisodeInfoUrl(string $showId, string $seasonId, string $episodeId): string
    {
        // THis %02d adds a trailing 0 in front of the number as a string, like 01 instead of 1
        return TOUTV_URL_EPISODE_INFO . $showId . "/s" . sprintf("%02d", $seasonId) . "e" . sprintf("%02d", $episodeId);
    }

    protected function getEpisodeInfoParameters(string $showId, string $seasonId, string $episodeId): array
    {
        return TOUTV_PARAMETERS_EPISODE_INFO;
    }

    //region New functions

    private function getEpisodeFileUrl(string $episodeId): string
    {
        return TOUTV_URL_EPISODE_FILE_INFO;
    }

    private function getEpisodeFileParameters(string $episodeId): array
    {
        $parameters = TOUTV_PARAMETERS_EPISODE_FILE_INFO;
        $parameters["idMedia"] = $episodeId;
        return $parameters;
    }

    private function getEpisodeDownloadUrl(): string
    {
        return TOUTV_URL_EPISODE_DOWNLOAD_INFO;
    }

    private function getEpisodeDownloadParameters(string $episodeId): array
    {
        $parameters = TOUTV_PARAMETERS_EPISODE_DOWNLOAD_INFO;
        $parameters["idMedia"] = $episodeId;
        return $parameters;
    }

    private function getEpisodeDownloadHeaders(): array
    {
        $headers = TOUTV_HEADERS_EPISODE_DOWNLOAD_INFO;
        $headers["Authorization"] = "";
        $headers["x-claims-token"] = "";
        $headers = array_merge(HTTP_DEFAULT_HEADERS, $headers);
        return $headers;
    }

    //endregion

    //endregion

}
