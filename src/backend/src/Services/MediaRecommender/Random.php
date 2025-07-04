<?php

declare(strict_types=1);

namespace App\Services\MediaRecommender;

use App\Models\MediaRecommender;

/**
 * Using PHP's random to order results
 */
class Random extends MediaRecommender
{
    public static function orderResults(int $amount, array $results): array
    {
        shuffle($results);
        return array_splice($results, 0, $amount);
    }
}
