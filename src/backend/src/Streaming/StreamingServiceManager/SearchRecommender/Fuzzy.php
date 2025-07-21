<?php

declare(strict_types=1);

namespace App\Streaming\StreamingServiceManager\SearchRecommender;

use App\Streaming\StreamingServiceManager\Classes\SearchRecommender;
use Fuse\Fuse;

/**
 * Using Fuzzy search to order search results
 */
final class Fuzzy extends SearchRecommender
{
    private static array $options = [
        "keys" => [],
        "threshold" => 1, // Be as lax as possible, as I would like as many results as possible, just order them
    ];

    #[\Override]
    public static function orderResults(
        string $query,
        array $results,
        array $keys = ["title"],
    ): array {
        self::$options["keys"] = $keys;
        $fuse = new Fuse($results, self::$options);
        $rawOrderedResults = $fuse->search($query);

        $orderedResults = [];

        foreach ($rawOrderedResults as $result) {
            array_push($orderedResults, $result["item"]);
        }

        return $orderedResults;
    }
}
