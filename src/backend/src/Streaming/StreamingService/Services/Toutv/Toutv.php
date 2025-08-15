<?php

declare(strict_types=1);

namespace App\Streaming\StreamingService\Services\Toutv;

use App\Config\Constants;
use App\Streaming\StreamingService\Services\Toutv\Config;
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

    public string $shortDescription = "";
    public string $fullDescription = "";
    public string $imageCard = "https://ici.tou.tv/images/share/toutv.png";
    public string $imageBackground = "https://ici.tou.tv/images/share/toutv-extra.png";

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

        // New API, but cannot return results that have less than 3 characters
        if (isset($response["results"])) {
            foreach ($response["results"] as $result) {
                $show = ObjectFactory::createShow();

                $show->id = $result["url"];
                $show->title = $result["title"];
                $show->shortDescription = $show->fullDescription =
                    $result["infoTitle"];

                $show->imageCard = $result["images"]["card"]["url"];

                if ($result["type"] == "Show") {
                    $results[] = $show;
                }
            }
        }

        // Old API, worse results but less strict on query length
        if (isset($response["result"])) {
            foreach ($response["result"] as $result) {
                $show = ObjectFactory::createShow();

                $show->id = $result["url"];
                $show->title = $result["title"];
                $show->shortDescription = $show->fullDescription =
                    $result["searchableText"]; // Totally false, this is not a description

                $show->imageCard = $result["image"]["url"];

                if ($result["type"] == "Show") {
                    $results[] = $show;
                }
            }
        }

        return $results;
    }

    private function parseRecommendationTypes(array $response): array
    {
        $types = [];

        foreach ($response["formats"] as $format) {
        }
    }

    #[\Override]
    public function retrieveRecommendationTypes(): array
    {
        $response = json_decode(
            RequestHelper::get(
                $this->getRecommendationTypesUrl(),
                $this->getRecommendationTypesParameters(),
                $this->getRecommendationTypesHeaders(),
            ),
            true,
        );

        return $this->parseRecommendations($response);
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

        foreach ($response["recommendations"]["items"] as $recommendation) {
            $show = ObjectFactory::createShow();

            $show->id = $recommendation["url"];
            $show->title = $recommendation["title"];
            $show->shortDescription = $recommendation["infoTitle"];
            $show->fullDescription = $recommendation["description"];

            $show->imageBackground =
                $recommendation["images"]["background"]["url"];
            $show->imageCard = $recommendation["images"]["card"]["url"];

            $recommendations[] = $show;
        }

        return $recommendations;
    }

    private function parseRecommendations(array $response): array
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

            $recommendations[] = $show;
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

        return $this->parseRecommendations($response);
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

        return $this->parseRecommendations($response);
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

        return $this->parseRecommendations($response);
    }

    private function parseNextEpisodeInSeasonRecommendation(
        Show $show,
        Season $season,
        Episode $episode,
        array $response,
    ): bool {
        // This loop is for the next episode in the same season
        foreach ($response["content"][0]["lineups"] as $s) {
            foreach ($s["items"] as $episodeKey => $e) {
                if ($s["seasonNumber"] === $season->number) {
                    if ($e["episodeNumber"] === $episode->number) {
                        if (isset($s["items"][$episodeKey + 1])) {
                            $nextEpisode = $s["items"][$episodeKey + 1];

                            $episode->id = (string) $nextEpisode["idMedia"];
                            $episode->title = $nextEpisode["title"];
                            $episode->number = $nextEpisode["episodeNumber"];
                            $episode->shortDescription = $episode->fullDescription =
                                $nextEpisode["description"] ?? "";
                            $episode->imageCard =
                                $nextEpisode["images"]["card"]["url"];

                            return true;
                        }
                    }
                }
            }
        }

        return false;
    }

    private function parseFirstEpisodeInNextSeasonRecommendation(
        Show $show,
        Season $season,
        Episode $episode,
        array $response,
    ): bool {
        // This loop is for retrieving the next season's first episode
        foreach ($response["content"][0]["lineups"] as $seasonKey => $s) {
            if ($s["seasonNumber"] === $season->number) {
                if (isset($response["content"][0]["lineups"][$seasonKey + 1])) {
                    $nextSeason =
                        $response["content"][0]["lineups"][$seasonKey + 1];
                    foreach ($nextSeason["items"] as $e) {
                        // Don't recommend Trailers
                        if ($e["type"] !== "Trailer") {
                            $season->number = $nextSeason["seasonNumber"];

                            $episode->id = (string) $e["idMedia"];
                            $episode->title = $e["title"];
                            $episode->number = $e["episodeNumber"];
                            $episode->shortDescription = $episode->fullDescription =
                                $e["description"] ?? "";
                            $episode->imageCard = $e["images"]["card"]["url"];

                            return true;
                        }
                    }
                }
            }
        }

        return false;
    }

    private function parseFirstEpisodeInFirstSeasonRecommendation(
        Show $show,
        Season $season,
        Episode $episode,
        array $response,
    ): bool {
        $firstSeason = $response["content"][0]["lineups"][0];

        $season->number = $firstSeason["seasonNumber"];

        // Returns the first episode of the show
        foreach ($firstSeason["items"] as $e) {
            // Don't recommend Trailers
            if ($e["type"] !== "Trailer") {
                $episode->id = (string) $e["idMedia"];
                $episode->title = $e["title"];
                $episode->number = $e["episodeNumber"];
                $episode->shortDescription = $episode->fullDescription =
                    $e["description"] ?? "";
                $episode->imageCard = $e["images"]["card"]["url"];

                return true;
            }
        }

        return false;
    }

    // This is totally wrong and I shouldn't do this, but the way I structured my functions made me do it
    #[\Override]
    public function retrieveNextEpisodeRecommendation(
        Show $show,
        Season $season,
        Episode $episode,
    ): Episode {
        $response = json_decode(
            RequestHelper::get(
                $this->getNextEpisodeRecommendationUrl(
                    $show,
                    $season,
                    $episode,
                ),
                $this->getNextEpisodeRecommendationParameters(
                    $show,
                    $season,
                    $episode,
                ),
                $this->getNextEpisodeRecommendationHeaders(
                    $show,
                    $season,
                    $episode,
                ),
            ),
            true,
        );

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
            $response,
        )
        ) {
            return $episode;
        }

        if ($this->parseFirstEpisodeInNextSeasonRecommendation(
            $show,
            $season,
            $episode,
            $response,
        )
        ) {
            return $episode;
        }

        if ($this->parseFirstEpisodeInFirstSeasonRecommendation(
            $show,
            $season,
            $episode,
            $response,
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

        // Set title, and if it is originally in a language other than french set it to original language
        $show->title = $response["originalTitle"] ?? $response["title"];
        $show->shortDescription = $show->fullDescription =
            $response["description"] ?? "";

        $releaseDate = $response["structuredMetadata"]["datePublished"] ?? null;
        $show->year = (int) explode("-", $releaseDate ? $releaseDate : "")[0]; // The first number being the year

        $show->imageBackground = $response["images"]["background"]["url"] ?? "";

        foreach ($response["content"][0]["lineups"] as $responseSeason) {
            $season = ObjectFactory::createSeason();
            $season->id = (string) $responseSeason["seasonNumber"]; // Toutv doesn't have a ID, except URL
            $season->title = $responseSeason["title"];
            $season->number = $responseSeason["seasonNumber"];

            $show->seasons[] = $season;
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

        foreach ($response["content"][0]["lineups"] as $responseSeason) {
            // Find the right season that matches the season's ID requested
            if ($responseSeason["seasonNumber"] === $season->number) {
                $season->number = $responseSeason["seasonNumber"];
                $season->id = (string) $season->number;
                $season->title = $responseSeason["title"];
                $season->fullDescription = $season->shortDescription =
                    $response["structuredMetadata"]["abstract"];

                foreach ($responseSeason["items"] as $responseEpisode) {
                    $episode = ObjectFactory::createEpisode();

                    $episode->id = (string) $responseEpisode["idMedia"];
                    $episode->title = $responseEpisode["title"];
                    $episode->number = $responseEpisode["episodeNumber"];
                    $episode->shortDescription = $episode->fullDescription =
                        $responseEpisode["description"] ?? "";
                    $episode->imageCard =
                        $responseEpisode["images"]["card"]["url"];

                    // Don't add Trailers
                    if ($responseEpisode["type"] !== "Trailer") {
                        $season->episodes[] = $episode;
                    }
                }

                break; // If it enters the if, that's the only time it will
            }
        }
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

        $episode->id = $response["content.mediaId"];
        $episode->title = $response["content.title"];
        $episode->number = (int) $response["content.episode"];

        if ($stream) {
            $this->getEpisodeStreamInfo($episode);
        }
    }

    // TODO: Find the right way to choose the right stream/drm
    // public function getEpisodeStreamingTechnology(Episode $episode)
    // {
    //     $fileParameters = $this->getEpisodeFileParameters($episode);

    //     $response = RequestHelper::get(
    //         $this->getEpisodeFileUrl($episode),
    //         $fileParameters,
    //         Constants::HTTP_DEFAULT_HEADERS,
    //     );

    //     $this->parseEpisodeFileInfo($episode, json_decode($response, true));

    // }

    private function getEpisodeStreamInfo(Episode $episode): void
    {
        $response = json_decode(
            RequestHelper::get(
                $this->getEpisodeFileUrl($episode),
                $this->getEpisodeFileParameters($episode),
                $this->getEpisodeFileHeaders($episode),
            ),
            true,
        );

        $this->parseEpisodeFileInfo($episode, $response);

        $response = json_decode(
            RequestHelper::get(
                $this->getEpisodeDownloadUrl($episode),
                $this->getEpisodeDownloadParameters($episode),
                $this->getEpisodeDownloadHeaders($episode),
            ),
            true,
        );

        $this->parseEpisodeDownloadStreamInfo($episode, $response);
    }

    private function parseStreamingTechnologies(
        Episode $episode,
        array $availableTechs,
    ) {
        // Parses the availableTechs for the available DRM and streaming techs
        foreach ($availableTechs as $streamingTechnology) {
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
    }

    private function parseEpisodeFileInfo(
        Episode $episode,
        array $response,
    ): void {
        // TODO: Actually choose a streaming technology and don't just pick dash with widevine

        $this->parseStreamingTechnologies(
            $episode,
            $response["availableTechs"],
        );

        $episode->id = $response["Metas"]["idMedia"];
        $episode->title = $response["Metas"]["Title"];
        $episode->number = (int) $response["Metas"]["SrcEpisode"];

        $episode->fullDescription =
            $response["Metas"]["Description"] ?:
            $response["Metas"]["ShortDescription"] ?:
            "";
        $episode->shortDescription = !empty(
            $response["Metas"]["ShortDescription"]
        )
            ? $response["Metas"]["ShortDescription"]
            : $episode->fullDescription;

        $episode->containsDrm = (bool) $response["Metas"]["isDrmActive"];
    }

    private function parseEpisodeDownloadStreamInfo(
        Episode $episode,
        array $response,
    ): void {
        $episode->url = $response["url"];
        $episode->urlHeaders = [];

        $episode->streamingTechnology->drmTechnology->licenseHeaders = array_merge(
            $this->getEpisodeDownloadHeaders($episode),
            Config::HEADERS_EPISODE_DOWNLOAD_LICENSE_INFO,
        );

        if ($episode->streamingTechnology->drmTechnology->name === "widevine") {
            foreach ($response["params"] as $parameter) {
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

    private function executeLoginAuthorize(): array
    {
        $response = RequestHelper::get(
            Config::LOGIN_URL,
            $this->getLoginParameters(),
            Constants::HTTP_DEFAULT_HEADERS,
            [
                CURLOPT_HEADER => true,
            ],
        );

        $body = $response["body"];

        // Retrieve the internal JSON as string
        preg_match(Config::LOGIN_SETTINGS_RE, $body, $matches);
        $settings = json_decode($matches[1], true);

        // Select stateProperties, which is in the transId
        $explodedStateProperties = explode("=", $settings["transId"]);
        $stateProperties = [
            $explodedStateProperties[0] => $explodedStateProperties[1],
        ];
        $formattedStateProperties = RequestHelper::format_parameters(
            $stateProperties,
            false,
        );
        $parameters = ["tx" => $formattedStateProperties];

        $headers = $response["headers"];
        $cookies = $headers["set-cookie"];

        return [$cookies, $parameters];
    }

    private function executeLoginSelfAsserted(
        array $cookies,
        array $parameters,
    ): array {
        $data = Config::LOGIN_SELF_ASSERTED_DATA;

        $data["email"] = $_ENV["TOUTV_EMAIL"];
        $data["password"] = $_ENV["TOUTV_PASSWORD"];

        $headers = Config::LOGIN_SELF_ASSERTED_HEADERS;
        $headers["Cookie"] = RequestHelper::format_cookies($cookies);

        $headers["X-CSRF-TOKEN"] = $cookies["x-ms-cpim-csrf"];

        $response = RequestHelper::post(
            Config::LOGIN_SELF_ASSERTED_URL,
            $parameters,
            $headers,
            [
                CURLOPT_HEADER => true,
            ],
            $data,
            "application/x-www-form-urlencoded",
        );

        $headers = $response["headers"];
        $cookies += $headers["set-cookie"];

        return [$cookies, $parameters];
    }

    private function executeLoginConfirmed(
        array $cookies,
        array $parameters,
    ): array {
        $headers = Config::LOGIN_CONFIRMED_HEADERS;
        $headers["Cookie"] = RequestHelper::format_cookies($cookies);

        $parameters["csrf_token"] = $cookies["x-ms-cpim-csrf"];

        $response = RequestHelper::get(
            Config::LOGIN_CONFIRMED_URL,
            $parameters,
            $headers,
            [
                CURLOPT_HEADER => true,
                CURLOPT_FOLLOWLOCATION => true,
            ],
        );

        $redirectUrl = $response["redirectUrl"];
        $headers = $response["headers"];
        $cookies = $headers["set-cookie"];

        print_r($redirectUrl);
        echo "<br>";
        echo "<br>";
        print_r($cookies);

        return [$cookies, $parameters];
    }

    private function login(): array
    {
        // GET_AUTHORISE
        [$cookies, $parameters] = $this->executeLoginAuthorize();

        // GET_SELF_ASSERTED
        [$cookies, $parameters] = $this->executeLoginSelfAsserted(
            $cookies,
            $parameters,
        );

        // GET_ACCESS_TOKEN_MS (Confirmed)
        [$cookies, $parameters] = $this->executeLoginConfirmed(
            $cookies,
            $parameters,
        );

        return [];
    }

    //endregion

    //region Specific values

    private function getSearchUrl(string $query, int $amount): string
    {
        if (strlen($query) < 2) {
            return Config::SECOND_URL_SEARCH;
        }
        return Config::URL_SEARCH;
    }

    private function getSearchParameters(string $query, int $amount): array
    {
        $parameters = [];

        if (strlen($query) < 2) {
            $parameters = Config::SECOND_PARAMETERS_SEARCH;

            // Because old search returns A LOT of not show results
            $parameters["pageSize"] = $amount * 10;
        } else {
            $parameters = Config::PARAMETERS_SEARCH;
            $parameters["pageSize"] = $amount;
        }

        $parameters["term"] = urlencode($query);

        return $parameters;
    }

    private function getSearchHeaders(string $query, int $amount): array
    {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    private function getRecommendationTypesUrl(): string
    {
        return Config::URL_RECOMMENDATION_TYPES;
    }

    private function getRecommendationTypesParameters(): array
    {
        return Config::PARAMETERS_RECOMMENDATION_TYPES;
    }

    private function getRecommendationTypesHeaders(): array
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
        $parameters = Config::PARAMETERS_SHOW_RECOMMENDATIONS;
        $parameters["pageSize"] = $amount;
        return $parameters;
    }

    private function getShowRecommendationsHeaders(
        Show $show,
        int $amount,
    ): array {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    private function getMovieRecommendationsUrl(int $amount): string
    {
        return Config::URL_MOVIES_RECOMMENDATIONS;
    }

    private function getMovieRecommendationsParameters(int $amount): array
    {
        $parameters = Config::PARAMETERS_MOVIES_RECOMMENDATIONS;
        $parameters["pageSize"] = $amount;
        return $parameters;
    }

    private function getMovieRecommendationsHeaders(int $amount): array
    {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    private function getSerieRecommendationsUrl(int $amount): string
    {
        return Config::URL_SERIES_RECOMMENDATIONS;
    }

    private function getSerieRecommendationsParameters(int $amount): array
    {
        return $this->getMovieRecommendationsParameters($amount);
    }

    private function getSerieRecommendationsHeaders(int $amount): array
    {
        return $this->getMovieRecommendationsHeaders($amount);
    }

    private function getDocumentaryRecommendationsUrl(int $amount): string
    {
        return Config::URL_DOCUMENTARIES_RECOMMENDATIONS;
    }

    private function getDocumentaryRecommendationsParameters(int $amount): array
    {
        return $this->getMovieRecommendationsParameters($amount);
    }

    private function getDocumentaryRecommendationsHeaders(int $amount): array
    {
        return $this->getMovieRecommendationsHeaders($amount);
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
        return Config::URL_SHOW_INFO . $show->id;
    }

    private function getShowParameters(Show $show): array
    {
        return Config::PARAMETERS_SHOW_INFO;
    }

    private function getShowHeaders(Show $show): array
    {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    private function getSeasonUrl(Show $show, Season $season): string
    {
        return Config::URL_SEASON_INFO . $show->id . "/s" . $season->number;
    }

    private function getSeasonParameters(Show $show, Season $season): array
    {
        return Config::PARAMETERS_SEASON_INFO;
    }

    private function getSeasonHeaders(Show $show, Season $season): array
    {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    private function getEpisodeUrl(
        Show $show,
        Season $season,
        Episode $episode,
    ): string {
        // THis %02d adds a trailing 0 in front of the number as a string, like 01 instead of 1
        return Config::URL_EPISODE_INFO .
            $show->id .
            "/s" .
            sprintf("%02d", $season->number) .
            "e" .
            sprintf("%02d", $episode->number);
    }

    private function getEpisodeParameters(
        Show $show,
        Season $season,
        Episode $episode,
    ): array {
        return Config::PARAMETERS_EPISODE_INFO;
    }

    private function getEpisodeHeaders(
        Show $show,
        Season $season,
        Episode $episode,
    ): array {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    //region New functions

    private function getEpisodeFileUrl(Episode $episode): string
    {
        return Config::URL_EPISODE_FILE_INFO;
    }

    private function getEpisodeFileParameters(Episode $episode): array
    {
        $parameters = Config::PARAMETERS_EPISODE_FILE_INFO;
        $parameters["idMedia"] = $episode->id;
        return $parameters;
    }

    private function getEpisodeFileHeaders(Episode $episode): array
    {
        return Constants::HTTP_DEFAULT_HEADERS;
    }

    private function getEpisodeDownloadUrl(Episode $episode): string
    {
        return Config::URL_EPISODE_DOWNLOAD_INFO;
    }

    private function getEpisodeDownloadParameters(Episode $episode): array
    {
        $parameters = Config::PARAMETERS_EPISODE_DOWNLOAD_INFO;
        $parameters["idMedia"] = $episode->id;
        return $parameters;
    }

    private function getEpisodeDownloadHeaders(Episode $episode): array
    {
        $headers = Config::HEADERS_EPISODE_DOWNLOAD_INFO;
        $headers["Authorization"] = "";
        $headers["x-claims-token"] = "";
        $headers = array_merge(Constants::HTTP_DEFAULT_HEADERS, $headers);
        return $headers;
    }

    private function getLoginParameters(): array
    {
        $parameters = Config::LOGIN_PARAMETERS;

        $parameters["redirect_uri"] = rawurlencode(Config::LOGIN_REDIRECT_URL);

        $parameters["nonce"] = rawurlencode(Config::GET_LOGIN_NONCE());

        $scope = "";

        foreach (Config::LOGIN_SCOPE as $s) {
            $scope .= $s . " ";
        }

        $parameters["scope"] = rawurlencode(rtrim($scope));

        $parameters["state"] = $parameters["state_value"] = rawurlencode(
            Config::GET_LOGIN_STATE(),
        );
        $parameters["code_challenge"] = rawurlencode(
            Config::GET_CODE_CHALLENGE(),
        );

        return $parameters;
    }

    //endregion

    //endregion
}
