<?php

declare(strict_types=1);

namespace App\Streaming\StreamingServiceManager\Classes;

abstract class SearchRecommender
{
    abstract public static function orderResults(string $query, array $results): array;
}
