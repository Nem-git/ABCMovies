<?php

declare(strict_types=1);

namespace App\Streaming\Classes;

use App\Streaming\StreamingTechnology\StreamingTechnology;

final class Episode
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
     * Season Number
     */
    public int $seasonNumber;
    /**
     * Show ID
     */
    public string $showId;
    /**
     * The streaming service's tag
     */
    public string $streamingServiceTag;

    /**
     * Download link. Not only used to show local download link, but also streaming service download link
     */
    public string $url;

    /**
     * Headers required to use the download link
     *
     * @var array<string, string>
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
