<?php

declare(strict_types=1);

namespace App\Streaming\StreamingServiceManager\Classes;

abstract class MediaRecommender
{
    abstract public static function orderResults(
        int $amount,
        array $results,
    ): array;
}
