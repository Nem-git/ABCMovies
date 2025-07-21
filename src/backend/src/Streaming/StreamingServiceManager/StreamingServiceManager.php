<?php

declare(strict_types=1);

namespace App\Streaming\StreamingServiceManager;

use Psr\Http\Message\ServerRequestInterface as Request;
use App\Helpers\SlimRequestParsingHelper;
use App\Streaming\StreamingServiceManager\Classes\SearchRecommender;
use App\Streaming\StreamingServiceManager\Classes\MediaRecommender;
use App\Factory\ObjectFactory;

final class StreamingServiceManager
{
    private SearchRecommender $searchRecommender;
    private MediaRecommender $mediaRecommender;

    private array $streamingServices = ["TOUTV"];

    public function __construct()
    {
        $this->searchRecommender = ObjectFactory::createSearchRecommender(
            "fuzzy",
        );
        $this->mediaRecommender = ObjectFactory::createMediaRecommender(
            "random",
        );
    }

    public function getSearchResults(Request $request, array $args): array
    {
        $searchResultsCriteria = SlimRequestParsingHelper::parseSearchCriteria(
            $request,
            $args,
        );

        return $this->executeSearchResults(
            $searchResultsCriteria["query"],
            $searchResultsCriteria["amount"],
        );
    }

    public function executeSearchResults(string $query, int $amount): array
    {
        $allResults = [];

        foreach ($this->streamingServices as $streamingServiceTag) {
            $streamingService = ObjectFactory::createStreamingService(
                $streamingServiceTag,
            );
            $allResults[] = $streamingService->executeSearchResults(
                $query,
                $amount,
            );
        }

        // To remove the classes and leave it as an associative array all the way for easier use with Fuse
        // also, the ... are to flatten the array, because for some reason it made an array of 1 containing results arrays
        $allResults = array_merge(
            ...json_decode(json_encode($allResults), true),
        );

        $orderedResults = $this->searchRecommender->orderResults(
            $query,
            $allResults,
        );

        return $orderedResults;
    }

    public function getMediaRecommendations(
        Request $request,
        array $args,
    ): array {
        $recommendationsCriteria = SlimRequestParsingHelper::parseRecommendationsCriteria(
            $request,
            $args,
        );

        return $this->executeMediaRecommendations(
            $recommendationsCriteria["amount"],
            $recommendationsCriteria["type"],
        );
    }

    public function executeMediaRecommendations(
        int $amount,
        string $type,
    ): array {
        $allResults = [];

        foreach ($this->streamingServices as $streamingServiceTag) {
            $streamingService = ObjectFactory::createStreamingService(
                $streamingServiceTag,
            );
            $allResults[] = $streamingService->executeMediaRecommendations(
                $amount,
                $type,
            );
        }

        $allResults = array_merge(
            ...json_decode(json_encode($allResults), true),
        );

        $orderedResults = $this->mediaRecommender->orderResults(
            $amount,
            $allResults,
        );

        return $orderedResults;
    }
}
