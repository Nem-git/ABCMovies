<?php

declare(strict_types=1);

namespace App\Helpers;

use Psr\Http\Message\ServerRequestInterface as Request;

require_once __DIR__ . "/../../config/constants.php";

class StreamingServiceHelper
{
    public static function parseSearchCriteria(Request $request, array $args): array
    {
        return [
            "query" => $args["query"] ?? "",
            "amount" => (int)($request->getQueryParams()["amount"] ?? DEFAULT_SEARCH_RESULTS_AMOUNT),
        ];
    }

    public static function parseShowRecommendationsCriteria(Request $request, array $args): array
    {
        return [
            "showId" => $args["show"] ?? "",
            "amount" => (int)($request->getQueryParams()["amount"] ?? DEFAULT_RECOMMENDATIONS_AMOUNT),
        ];
    }

    public static function parseRecommendationsCriteria(Request $request, array $args): array
    {
        return [
            "amount" => (int)($request->getQueryParams()["amount"] ?? DEFAULT_RECOMMENDATIONS_AMOUNT),
        ];
    }

    public static function parseNextRecommendationCriteria(Request $request, array $args): array
    {
        return [
            "showId" => $args["show"] ?? "",
            "seasonId" => $args["season"] ?? "",
            "episodeId" => $args["episode"] ?? "",
        ];
    }

    public static function parseShowInfoCriteria(Request $request, array $args): array
    {
        return [
            "showId" => $args["show"] ?? "",
        ];
    }

    public static function parseSeasonInfoCriteria(Request $request, array $args): array
    {
        return [
            "showId" => $args["show"] ?? "",
            "seasonId" => $args["season"] ?? "",
        ];
    }

    public static function parseEpisodeInfoCriteria(Request $request, array $args): array
    {
        return [
            "showId" => $args["show"] ?? "",
            "seasonId" => $args["season"] ?? "",
            "episodeId" => $args["episode"] ?? "",
        ];
    }

    public static function parseEpisodeStreamCriteria(Request $request, array $args): array
    {
        return [
            "showId" => $args["show"] ?? "",
            "seasonId" => $args["season"] ?? "",
            "episodeId" => $args["episode"] ?? "",
        ];
    }

    public static function parseEpisodeInitSegmentCriteria(Request $request, array $args): array
    {
        $originalInitBaseUrl = base64_decode($args["encodedBaseUrl"], true) ?? "";
        $originalInitUrlWithoutParameters = $originalInitBaseUrl . $args["segmentPath"];
        $originalInitUrl = $originalInitUrlWithoutParameters . RequestHelper::format_parameters($request->getQueryParams() ?? []);
        return [
            "originalInitUrl" => $originalInitUrl ?? "",
        ];
    }

    public static function parseEpisodeMediaSegmentCriteria(Request $request, array $args): array
    {
        $originalMediaBaseUrl = base64_decode($args["encodedBaseUrl"], true) ?? "";
        $originalMediaUrlWithoutParameters = $originalMediaBaseUrl . $args["segmentPath"];
        $originalMediaUrl = $originalMediaUrlWithoutParameters . RequestHelper::format_parameters($request->getQueryParams() ?? []);
        return [
            "originalInitUrl" => base64_decode($args["encodedInitUrl"]) ?? "",
            "originalMediaUrl" => $originalMediaUrl ?? "",

            "showId" => $args["show"] ?? "",
            "seasonId" => $args["season"] ?? "",
            "episodeId" => $args["episode"] ?? "",
        ];
    }

    public static function getStreamUrl(string $streamingServiceTag, string $showId, string $seasonId, string $episodeId, string $tech): string
    {
        return PHP_URL_BACKEND . join("/", [strtolower($streamingServiceTag), $showId, $seasonId, $episodeId, STREAMING_TECH[$tech]]);
    }

    public static function getEpisodeDatabaseIdentifier(string $streamingServiceTag, string $showId, string $seasonId, string $episodeId): string
    {
        return join("/", [strtolower($streamingServiceTag), $showId, $seasonId, $episodeId]);
    }

}
