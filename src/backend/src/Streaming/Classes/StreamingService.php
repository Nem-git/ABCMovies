<?php

declare(strict_types=1);

namespace App\Streaming\Classes;

final class StreamingService
{
    /**
     * Show unique identifier (In the streaming service)
     */
    public string $id;
    /**
     * Show title (Ex: La petite vie)
     */
    public string $title;
    /**
     * Tag, like DNSP, TOUTV, NOOVO, CRAV, NTFL
     */
    /**
     * Show long form description
     */
    public string $description;
    /**
     * Card image URL
     */
    public string $imageCard;
    /**
     * Background image URL
     */
    public string $imageBackground;

    /**
     * Shows in the streaming service
     */
    public array $shows;
}
