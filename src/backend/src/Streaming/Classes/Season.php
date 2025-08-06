<?php

declare(strict_types=1);

namespace App\Streaming\Classes;

final class Season
{
    /**
     * Season unique identifier (In the streaming service)
     */
    public string $id;
    /**
     * Season title (Ex: Le voyage à Plattsburg)
     */
    public string $title;
    /**
     * Season number
     */
    public int $number;
    /**
     * Season long form description
     */
    public string $fullDescription;
    /**
     * Season short form description
     */
    public string $shortDescription;

    /**
     * Show ID
     */
    public string $showId;
    /**
     * The streaming service's tag
     */
    public string $streamingServiceTag;

    /**
     * Entirety of episodes in the season
     *
     * @var Episode[]
     */
    public array $episodes;
}
