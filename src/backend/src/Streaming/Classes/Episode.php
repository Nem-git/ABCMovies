<?php

declare(strict_types=1);

namespace App\Streaming\Classes;

use App\Streaming\StreamingTechnology\StreamingTechnology;

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
     * Download link
     */
    public string $url;

    /**
     * Headers required to use the download link
     */
    public array $urlHeaders;

    /**
     * The chosen streaming technology using settings in constants
     */
    public StreamingTechnology $streamingTechnology;

    /**
     * Wether or not the episode's video is DRM-protected
     */
    public bool $containsDrm;
}
