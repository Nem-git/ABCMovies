<?php

declare(strict_types=1);

class Episode {
    
    // Removed for now, because it didn't make sense for toutv
    // function __construct(string $id) {
    //     $this->id = $id;
    // }


    /** Episode's unique identifier (In the streaming service) */
    public string $id;
    /** Episode's title (Ex: Le voyage à Plattsburg) */
    public string $title;
    /** Episode number */
    public int $number;
    /** Card image URL */
    public string $imageCard;
    /** Episode's long form description */
    public string $fullDescription;
    /** Episode's short form description */
    public string $shortDescription;
}