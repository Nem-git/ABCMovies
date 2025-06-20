<?php

declare(strict_types=1);

namespace App\Services;

use Psr\Http\Message\ServerRequestInterface as Request;
use App\Helpers\StreamingServiceHelper;
use App\Models\ObjectFactory;
use App\Models\SearchRecommender;

class StreamingServiceManager
{
    private SearchRecommender $searchRecommender;

    private array $streamingServices = [
        "TOUTV",
    ];

    public function __construct()
    {
        $this->searchRecommender = ObjectFactory::createSearchRecommender("fuzzy");
    }


    public function getSearchResults(Request $request, array $args)
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




}
