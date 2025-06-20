<?php

declare(strict_types=1);

namespace App\Models;

abstract class SearchRecommender
{
    abstract public static function orderResults(string $query, array $results): array;
}
