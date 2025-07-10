<?php

declare(strict_types=1);

namespace App\Helpers;

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
