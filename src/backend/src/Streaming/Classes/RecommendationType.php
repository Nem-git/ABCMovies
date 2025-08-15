<?php

declare(strict_types=1);

namespace App\Streaming\Classes;

use App\Streaming\StreamingService\StreamingService;

final class RecommendationType
{
    /**
     * Recommendation type's unique identifier (In the streaming service)
     */
    public string $id;
    /**
     * Recommendation type's name
     */
    public string $name;
    /**
     * Short form description of the recommendation type
     */
    public string $shortDescription;
    /**
     * Long form description of the recommendation type
     */
    public string $fullDescription;
    /**
     * The streaming service's tag
     */
    public string $streamingServiceTag;
}
