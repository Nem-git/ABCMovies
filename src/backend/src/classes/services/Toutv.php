<?php

use Psr\Http\Message\ResponseInterface as Response;
use Psr\Http\Message\ServerRequestInterface as Request;

require_once __DIR__ . "/../StreamingService.php";

/** Tou.TV, le site de streaming payant de Radio-Canada */
class Toutv extends StreamingService {

    protected string $name = "Tou.TV";
    protected string $tag = "TOUTV";
    
    //region Search
    
    function parseSearchResults(array $response): array {
        $results = [];

        foreach ($response["results"] as $result) {
            
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

    function getSearchResults(Request $request, Response $response, array $args): string {

        $query = $args["query"] ?? "*";
        $amount = (int)($request->getQueryParams()["amount"] ?? 20);

        // TODO: Filter inputs

        $parameters = TOUTV_PARAMETERS_SEARCH;

        $parameters["pageSize"] = $amount;
        $parameters["term"] = $query;

        // TODO: Validate that the request worked

        $response = get_request(TOUTV_URL_SEARCH, HTTP_DEFAULT_HEADERS, $parameters);
        
        $results = self::parseSearchResults(json_decode($response, true));
        
        return json_encode($results, JSON_PRETTY_PRINT); # TODO: Remove pretty print, just for debugging
        
    }

    //endregion
   
    //region Show info

    function parseShowInfo(Show $show, array $ssResponse) {

        # Set title, and if it is originally in a language other than french set it to original language
        $show->title = $ssResponse["originalTitle"] ?? $ssResponse["title"];
        $show->shortDescription = $show->fullDescription = $ssResponse["description"] ?? "";
        
        $releaseDate = $ssResponse["structuredMetadata"]["datePublished"] ?? null;
        $show->year = (int)explode("-", $releaseDate ? $releaseDate : "")[0]; // The first number being the year

        $show->imageBackground = $ssResponse["images"]["background"]["url"];

        foreach ($ssResponse["content"][0]["lineups"] as $ssResponseSeason) {
            $season = new Season($ssResponseSeason["seasonNumber"]);

            $show->seasons[] = $season;
        }
    }

    function getShowInfo(Request $request, Response $response, array $args) {

        $showId = $args["show"] ?? "";

        $show = new Show($showId);

        $ssResponse = get_request(TOUTV_URL_SHOW_INFO . $show->id, HTTP_DEFAULT_HEADERS, TOUTV_PARAMETERS_SHOW_INFO);

        # TODO: Add verifications to make sure the request has a valid output

        $ssResponse = json_decode($ssResponse, true);

        self::parseShowInfo($show, $ssResponse);

        return json_encode($show, JSON_PRETTY_PRINT); # TODO: Remove pretty print, just for debugging
       
    }

    //endregion

    //region Season info

    function parseSeasonInfo(Season $season, array $ssResponse) {

        foreach ($ssResponse["content"][0]["lineups"] as $ssResponseSeason) {
            
            # Find the right season that matches the season's ID requested
            if ($ssResponseSeason["seasonNumber"] === (int)$season->id) {

                $season->title = $ssResponseSeason["title"];
                $season->number = $season->id;
                $season->fullDescription = $season->shortDescription = $ssResponse["structuredMetadata"]["abstract"];
                
                # Still not sure if episode shoudl be in Season, but I think it's best to keep it that way for now
                foreach ($ssResponseSeason["items"] as $ssResponseEpisode) {
                    $episode = new Episode((string)$ssResponseEpisode["idMedia"]);

                    $episode->title = $ssResponseEpisode["title"];
                    $episode->number = $ssResponseEpisode["episodeNumber"];
                    $episode->shortDescription = $episode->fullDescription = $ssResponseEpisode["description"] ?? "";
                    $episode->imageCard = $ssResponseEpisode["images"]["card"]["url"];

                    # Don't add Trailers
                    if ($ssResponseEpisode["type"] !== "Trailer") { $season->episodes[] = $episode; }

                }

                break;
            }
        }
    }

    function getSeasonInfo(Request $request, Response $response, array $args) {

        $showId = $args["show"] ?? "";
        $seasonId = $args["season"] ?? "";

        $season = new Season($seasonId);

        $ssResponse = get_request(TOUTV_URL_SEASON_INFO . $showId . "/s" . $seasonId, HTTP_DEFAULT_HEADERS, TOUTV_PARAMETERS_SEASON_INFO);

        # TODO: Add verifications to make sure the request has a valid output

        $ssResponse = json_decode($ssResponse, true);

        self::parseSeasonInfo($season, $ssResponse);

        return json_encode($season, JSON_PRETTY_PRINT); # TODO: Remove pretty print, just for debugging
       
    }

    //endregion

    //region Episode info

    function parseEpisodeInfo(Episode $episode, array $ssResponse) {

        $episode->id = $ssResponse["idFichierToutv"];
        $episode->title = $ssResponse["emission"];
        $episode->number = (int)$ssResponse["episode"];
    }

    function parseEpisodeFileInfo(Episode $episode, array $ssResponse) {
        
        $episode->id = $ssResponse["Metas"]["idMedia"];
        $episode->title = $ssResponse["Metas"]["Title"];
        $episode->number = (int)$ssResponse["Metas"]["SrcEpisode"];

        $episode->fullDescription = $ssResponse["Metas"]["Description"];
        $episode->shortDescription = !empty($ssResponse["Metas"]["ShortDescription"]) ? $ssResponse["Metas"]["ShortDescription"] : $episode->fullDescription;
    }

    function parseEpisodeDownloadInfo(Episode $episode, array $ssResponse) {

        $downloadInfo = [
            "mpdUrl" => null,
            "license" => null,
            "token" => null
        ];

        $downloadInfo["mpdUrl"] = $ssResponse["url"];

        foreach ($ssResponse["params"] as $param) {

            if ($param["name"] === "widevineLicenseUrl") {
                $downloadInfo["license"] = $param["value"];
            }
            if ($param["name"] === "widevineAuthToken") {
                $downloadInfo["token"] = $param["value"];
            }
        }

        

    }

    function getEpisodeInfo(Request $request, Response $response, array $args) {

        $showId = $args["show"] ?? "";
        $seasonId = $args["season"] ?? "";
        $episodeId = $args["episode"] ?? "";

        $episode = new Episode();
        
        $ssResponse = get_request(TOUTV_URL_EPISODE_INFO . $showId . "/s" . sprintf("%02d", $seasonId) . "e" . sprintf("%02d", $episodeId), HTTP_DEFAULT_HEADERS, TOUTV_PARAMETERS_EPISODE_INFO);

        # TODO: Add verifications to make sure the request has a valid output

        $ssResponse = json_decode($ssResponse, true);
        self::parseEpisodeInfo($episode, $ssResponse);

        $parameters = TOUTV_PARAMETERS_EPISODE_FILE_INFO;
        $parameters["idMedia"] = $episode->id;
        
        $ssResponse = get_request(TOUTV_URL_EPISODE_FILE_INFO, HTTP_DEFAULT_HEADERS, $parameters);

        # TODO: Add verifications to make sure the request has a valid output

        $ssResponse = json_decode($ssResponse, true);

        self::parseEpisodeFileInfo($episode, $ssResponse);

        $headers = HTTP_DEFAULT_HEADERS;

        $headers["x-claims-token"] = ""; // TODO: Retrieve the token from the database
        $headers["Authorization"] = ""; // TODO: Retrieve the Auth token from the database
        
        $ssResponse = get_request(TOUTV_URL_EPISODE_DOWNLOAD_INFO, $headers, TOUTV_PARAMETERS_EPISODE_DOWNLOAD_INFO);

        // TODO: Add verficiation to make sure the request has a valid output

        $ssResponse = json_decode($ssResponse, true);

        self::parseEpisodeDownloadInfo($episode, $ssResponse);

        return json_encode($episode, JSON_PRETTY_PRINT); # TODO: Remove pretty print, just for debugging
       
    }

    //endregion



    function getShowRecommendations(Request $request, Response $response, array $args) {
        return "";
    }
 




}