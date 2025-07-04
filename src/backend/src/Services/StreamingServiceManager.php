<?php

declare(strict_types=1);

namespace App\Services;

use Psr\Http\Message\ServerRequestInterface as Request;
use App\Helpers\StreamingServiceHelper;
use App\Models\ObjectFactory;
use App\Models\SearchRecommender;
use App\Models\MediaRecommender;

class StreamingServiceManager
{
    private SearchRecommender $searchRecommender;
    private MediaRecommender $mediaRecommender;

    private array $streamingServices = [
        "TOUTV",
    ];

    public function __construct()
    {
        $this->searchRecommender = ObjectFactory::createSearchRecommender("fuzzy");
        $this->mediaRecommender = ObjectFactory::createMediaRecommender("random");
    }


    public function getSearchResults(Request $request, array $args): array
    {
        $searchResultsCriteria = StreamingServiceHelper::parseSearchCriteria($request, $args);

        return $this->executeSearchResults(
            $searchResultsCriteria["query"],
            $searchResultsCriteria["amount"],
        );
    }

    public function executeSearchResults(string $query, int $amount): array
    {
        $allResults = [];

        foreach ($this->streamingServices as $streamingServiceTag) {
            $streamingService = ObjectFactory::createStreamingService($streamingServiceTag);
            $allResults[] = $streamingService->executeSearchResults($query, $amount);
        }

        // To remove the classes and leave it as an associative array all the way for easier use with Fuse
        // also, the ... are to flatten the array, because for some reason it made an array of 1 containing results arrays
        $allResults = array_merge(...json_decode(json_encode($allResults), true));

        $orderedResults = $this->searchRecommender->orderResults($query, $allResults);

        return $orderedResults;
    }

    public function getMediaRecommendations(Request $request, array $args): array
    {
        $recommendationsCriteria = StreamingServiceHelper::parseRecommendationsCriteria($request, $args);

        return $this->executeMediaRecommendations(
            $recommendationsCriteria["amount"],
            $recommendationsCriteria["type"],
        );
    }

    public function executeMediaRecommendations(int $amount, string $type): array
    {
        $allResults = [];

        foreach ($this->streamingServices as $streamingServiceTag) {
            $streamingService = ObjectFactory::createStreamingService($streamingServiceTag);
            $allResults[] = $streamingService->executeMediaRecommendations($amount, $type);
        }

        $allResults = array_merge(...json_decode(json_encode($allResults), true));

        $orderedResults = $this->mediaRecommender->orderResults($amount, $allResults);

        return $orderedResults;
    }


}
