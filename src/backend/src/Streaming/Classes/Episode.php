<?php

declare(strict_types=1);

namespace App\Streaming\Classes;

class Episode
{
    /**
     * Episode's unique identifier (In the streaming service)
     */
    public string $id;
    /**
     * Episode's title (Ex: Le voyage à Plattsburg)
     */
    public string $title;
    /**
     * Episode number
     */
    public int $number;
    /**
     * Episode's long form description
     */
    public string $fullDescription;
    /**
     * Episode's short form description
     */
    public string $shortDescription;
    /**
     * Card image URL
     */
    public string $imageCard;

    /**
     * The streaming service's name
     */
    public string $provider;

    /**
     * MPD link
     */
    public string $url;
}
