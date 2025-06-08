<?php

declare(strict_types=1);

namespace App\Models;

class Show
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
     * Release year
     */
    public int $year;
    /**
     * Show long form description
     */
    public string $fullDescription;
    /**
     * Show short form description
     */
    public string $shortDescription;
    /**
     * Card image URL
     */
    public string $imageCard;
    /**
     * Background image URL
     */
    public string $imageBackground;

    /**
     * Seasons in the show
     */
    public array $seasons;
}
