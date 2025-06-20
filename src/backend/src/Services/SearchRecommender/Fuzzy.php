<?php

declare(strict_types=1);

namespace App\Services\SearchRecommender;

use App\Models\SearchRecommender;
use Fuse\Fuse;

/**
 * Using Fuzzy search to order search results
 */
class Fuzzy extends SearchRecommender
{
    private static array $options = [
        "keys" => [],
        "threshold" => 1,
    ];

    public static function orderResults(string $query, array $results, array $keys = ["title"]): array
    {
        self::$options["keys"] = $keys;

        $fuse = new Fuse($results, self::$options);

        $orderedResults = $fuse->search($query);

        return $orderedResults;
    }
}
