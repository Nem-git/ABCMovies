<?php

declare(strict_types=1);

require_once __DIR__ . "/../constants.php";
require_once __DIR__ . "/Episode.php";
require_once __DIR__ . "/Season.php";
require_once __DIR__ . "/Show.php";


require_once __DIR__ . "/../utilities.php";

header("Content-Type: application/json; charset=utf-8");

class StreamingService {

    /** Streaming service's name */
    protected string $name;
    /** Streaming service's abreviation (EX: DSNP) */
    protected string $tag;

}