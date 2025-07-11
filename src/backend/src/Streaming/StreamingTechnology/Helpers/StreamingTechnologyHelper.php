<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology\Helpers;

class StreamingTechnologyHelper
{
    public static function reconstructUrlFromArray(string $scheme = "https", array $urlPath = [])
    {
        $newUrl = $scheme . "://";

        $joinedUrlPath = join("/", $urlPath);

        $newUrl .= $joinedUrlPath;

        return $newUrl;
    }
}
