<?php

declare(strict_types=1);

namespace App\Services\StreamingServices;

use App\Models\DownloadInfo;
use App\Services\StreamingService;
use App\Models\Show;
use App\Models\Season;
use App\Models\Episode;

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

            $show = new Show();

            $show->id = $result["url"];
            $show->title = $result["title"];
            $show->shortDescription = $show->fullDescription = $result["infoTitle"];

            // Replace the placeholder with teh newly formatted show title and add the parameters
            $show->imageCard = $result["images"]["card"]["url"];

            $show->provider = $this->tag;

            if ($result["type"] == "Show") {
                $results[] = $show;
            }
        }

        return $results;
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
            $season = new Season();
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
                    $episode = new Episode();

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
        return new DownloadInfo(); // Can't get info with the current request, so waiting for Optional
    }

    private function parseEpisodeFileInfo(Episode $episode, array $ssResponse): void
    {
        $episode->id = $ssResponse["Metas"]["idMedia"];
        $episode->title = $ssResponse["Metas"]["Title"];
        $episode->number = (int)$ssResponse["Metas"]["SrcEpisode"];

        $episode->fullDescription = $ssResponse["Metas"]["Description"];
        $episode->shortDescription = !empty($ssResponse["Metas"]["ShortDescription"]) ? $ssResponse["Metas"]["ShortDescription"] : $episode->fullDescription;
    }

    public function parseEpisodeDownloadStreamInfo(Episode $episode, array $ssResponse, $downloadInfo): void
    {
        $downloadInfo->licenseHeaders = array_merge(HTTP_DEFAULT_HEADERS, TOUTV_HEADERS_EPISODE_DOWNLOAD_LICENSE_INFO);

        //echo json_encode($ssResponse, JSON_PRETTY_PRINT);

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

    protected function getSearchUrl(): string
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

    protected function getShowInfoUrl(string $showId): string
    {
        return TOUTV_URL_SHOW_INFO . $showId;
    }

    protected function getShowInfoParameters(): array
    {
        return TOUTV_PARAMETERS_SHOW_INFO;
    }

    protected function getSeasonInfoUrl(string $showId, string $seasonId): string
    {
        return TOUTV_URL_SEASON_INFO . $showId . "/s" . $seasonId;
    }

    protected function getSeasonInfoParameters(): array
    {
        return TOUTV_PARAMETERS_SEASON_INFO;
    }

    protected function getEpisodeInfoUrl(string $showId, string $seasonId, string $episodeId): string
    {
        return TOUTV_URL_EPISODE_INFO . $showId . "/s" . sprintf("%02d", $seasonId) . "e" . sprintf("%02d", $episodeId);
    }

    protected function getEpisodeInfoParameters(): array
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
        $headers["Authorization"] = ""; // TODO: Implement a way to ask the DB for the tokens
        $headers["x-claims-token"] = ""; // TODO: ^^
        $headers = array_merge(HTTP_DEFAULT_HEADERS, $headers);
        return $headers;
    }


    public function getEpisodeDownloadInfoOptional(Episode $episode, DownloadInfo $downloadInfo): void
    {
        $fileParameters = $this->getEpisodeFileParameters($episode->id);
        $ssResponse = $this->request->get($this->getEpisodeFileUrl($episode->id), HTTP_DEFAULT_HEADERS, $fileParameters);
        $this->parseEpisodeFileInfo($episode, json_decode($ssResponse, true));

        $headers = $this->getEpisodeDownloadHeaders();
        $downloadParameters = $this->getEpisodeDownloadParameters($episode->id);
        $ssResponse = $this->request->get($this->getEpisodeDownloadUrl(), $headers, $downloadParameters);
        $this->parseEpisodeDownloadStreamInfo($episode, json_decode($ssResponse, true), $downloadInfo);
    }

    //endregion

    //endregion

}
