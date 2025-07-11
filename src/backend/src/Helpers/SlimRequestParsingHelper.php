<?php

declare(strict_types=1);

namespace App\Helpers;

use Psr\Http\Message\ServerRequestInterface as Request;
use App\Config\Constants;

class SlimRequestParsingHelper
{
    public static function parseSearchCriteria(Request $request, array $args): array
    {
        return [
            "query" => $args["query"] ?? "",
            "amount" => (int)($request->getQueryParams()["amount"] ?? Constants::DEFAULT_SEARCH_RESULTS_AMOUNT),
        ];
    }

    public static function parseShowRecommendationsCriteria(Request $request, array $args): array
    {
        return [
            "showId" => $args["show"] ?? "",
            "amount" => (int)($request->getQueryParams()["amount"] ?? Constants::DEFAULT_RECOMMENDATIONS_AMOUNT),
        ];
    }

    public static function parseRecommendationsCriteria(Request $request, array $args): array
    {
        return [
            "amount" => (int)($request->getQueryParams()["amount"] ?? Constants::DEFAULT_RECOMMENDATIONS_AMOUNT),
            "type" => $args["type"] ?? "",
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

    public static function parseEpisodeVideoCriteria(Request $request, array $args): array
    {
        $streamingTechnology = "";

        if (isset($args["streamingTechnology"])) {
            $streamingTechnology = Constants::WORD_TO_STREAMING_TECH[$args["streamingTechnology"]] ?? "";
        }

        if (isset($args["filename"])) {
            $streamingTechnology = Constants::WORD_TO_STREAMING_TECH[$args["filename"]] ?? "";
        }

        return [
            "showId" => $args["show"] ?? "",
            "seasonId" => $args["season"] ?? "",
            "episodeId" => $args["episode"] ?? "",
            "streamingTechnology" => $streamingTechnology,
        ];
    }
}
