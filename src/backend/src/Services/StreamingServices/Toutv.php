<?php

namespace App\Services\StreamingServices;

use App\Helpers\RequestHelper;
use Psr\Http\Message\ResponseInterface as Response;
use Psr\Http\Message\ServerRequestInterface as Request;
use App\Services\StreamingService;
use App\Models\Show;
use App\Models\Season;
use App\Models\Episode;

/**
 * Tou.TV, le site de streaming payant de Radio-Canada
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

            $show = new Show($result["url"]);

            $show->title = $result["title"];

            // Replace the placeholder with teh newly formatted show title and add the parameters
            $show->imageCard = $result["images"]["card"]["url"];

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

        foreach ($ssResponse["content"][0]["lineups"] as $ssResponseSeason) {
            $season = new Season();
            $season->id = $ssResponseSeason["seasonNumber"];

            $show->seasons[] = $season;
        }
    }

    protected function parseSeasonInfo(Season $season, array $ssResponse): void
    {
        foreach ($ssResponse["content"][0]["lineups"] as $ssResponseSeason) {

            // Find the right season that matches the season's ID requested
            if ($ssResponseSeason["seasonNumber"] === (int)$season->id) {

                $season->title = $ssResponseSeason["title"];
                $season->number = $season->id;
                $season->fullDescription = $season->shortDescription = $ssResponse["structuredMetadata"]["abstract"];

                // Still not sure if episode should be in Season, but I think it's best to keep it that way for now
                foreach ($ssResponseSeason["items"] as $ssResponseEpisode) {
                    $episode = new Episode((string)$ssResponseEpisode["idMedia"]);

                    $episode->title = $ssResponseEpisode["title"];
                    $episode->number = $ssResponseEpisode["episodeNumber"];
                    $episode->shortDescription = $episode->fullDescription = $ssResponseEpisode["description"] ?? "";
                    $episode->imageCard = $ssResponseEpisode["images"]["card"]["url"];

                    // Don't add Trailers
                    if ($ssResponseEpisode["type"] !== "Trailer") {
                        $season->episodes[] = $episode;
                    }
                }
                break;
            }
        }
    }

    protected function parseEpisodeInfo(Episode $episode, array $ssResponse): void
    {
        $episode->id = $ssResponse["idFichierToutv"];
        $episode->title = $ssResponse["emission"];
        $episode->number = (int)$ssResponse["episode"];
    }

    private function parseEpisodeFileInfo(Episode $episode, array $ssResponse): void
    {
        $episode->id = $ssResponse["Metas"]["idMedia"];
        $episode->title = $ssResponse["Metas"]["Title"];
        $episode->number = (int)$ssResponse["Metas"]["SrcEpisode"];

        $episode->fullDescription = $ssResponse["Metas"]["Description"];
        $episode->shortDescription = !empty($ssResponse["Metas"]["ShortDescription"]) ? $ssResponse["Metas"]["ShortDescription"] : $episode->fullDescription;
    }

    private function parseEpisodeDownloadInfo(Episode $episode, array $ssResponse): array
    {

        $dlInfo = [
            "mpdUrl" => null,
            "licenseUrl" => null,
            "token" => null
        ];

        $dlInfo["mpdUrl"] = $ssResponse["url"];

        foreach ($ssResponse["params"] as $param) {

            if ($param["name"] === "widevineLicenseUrl") {
                $dlInfo["licenseUrl"] = $param["value"];
            }
            if ($param["name"] === "widevineAuthToken") {
                $dlInfo["token"] = $param["value"];
            }
        }

        return $dlInfo;
    }

    //endregion

    //region Get TOU.TV values

    protected function getSearchUrl(): string
    {
        return TOUTV_URL_SEARCH;
    }

    protected function getSearchParameters(string $query, int $amount): array
    {
        $params = TOUTV_PARAMETERS_SEARCH;
        $params["pageSize"] = $amount;
        $params["term"] = $query;
        return $params;
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

    private function getEpisodeFileUrl(string $episodeId): string
    {
        return TOUTV_URL_EPISODE_FILE_INFO;
    }

    private function getEpisodeFileParameters(string $episodeId): array
    {
        $params = TOUTV_PARAMETERS_EPISODE_FILE_INFO;
        $params["idMedia"] = $episodeId;
        return $params;
    }

    private function getEpisodeDownloadUrl(): string
    {
        return TOUTV_URL_EPISODE_DOWNLOAD_INFO;
    }

    private function getEpisodeDownloadParameters(string $episodeId): array
    {
        $params = TOUTV_PARAMETERS_EPISODE_DOWNLOAD_INFO;
        $params["idMedia"] = $episodeId;
        return $params;
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

    //region Overrides

    public function getEpisodeInfo(Request $request, Response $response, array $args): Episode
    {
        $episode = parent::getEpisodeInfo($request, $response, $args);

        $fileParams = $this->getEpisodeFileParameters($episode->id);
        $ssResponse = $this->request->get($this->getEpisodeFileUrl($episode->id), HTTP_DEFAULT_HEADERS, $fileParams);
        $this->parseEpisodeFileInfo($episode, json_decode($ssResponse, true));

        $headers = $this->getEpisodeDownloadHeaders();
        $dlParams = $this->getEpisodeDownloadParameters($episode->id);
        $ssResponse = $this->request->get($this->getEpisodeDownloadUrl(), $headers, $dlParams);
        $dlInfo = $this->parseEpisodeDownloadInfo($episode, json_decode($ssResponse, true));

        $pssh = $this->widevine->get_pssh($dlInfo["mpdUrl"]);

        $headers = array_merge(HTTP_DEFAULT_HEADERS, TOUTV_HEADERS_EPISODE_DOWNLOAD_LICENSE_INFO);
        $headers["x-dt-auth-token"] = $dlInfo["token"];

        $decryptionKeys = $this->widevine->get_decryption_keys($pssh, $dlInfo["licenseUrl"], $headers);

        // Create the MPD link and add it to the output
        $episode->url = $request->getUri() . "/"; // TODO: Improve the link creation, it doesn't look right

        return $episode;
    }

    //endregion

}
