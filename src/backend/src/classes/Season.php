<?php

declare(strict_types=1);

class Season {
    
    function __construct(string $id) {
        $this->id = $id;
    }

    /** Season unique identifier (In the streaming service) */
    public string $id;
    /** Season title (Ex: Le voyage à Plattsburg) */
    public string $title;
    /** Season number */
    public int $number;
    /** Season long form description */
    public string $fullDescription;
    /** Season short form description */
    public string $shortDescription;
    
    /** Entirety of episodes in the season */
    public array $episodes;
}