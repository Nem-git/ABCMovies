<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology\Dash\Helpers;

use Psr\Http\Message\ServerRequestInterface as Request;
use App\Streaming\StreamingTechnology\Helpers\StreamingTechnologyHelper;

class DashHelper
{
    public static function parseDashSegmentCriteria(
        Request $request,
        array $args,
    ): array {
        // Whether it is an init or media segment
        $segmentType = strtolower(array_shift($args));

        // Gives the adaptation set unique ID
        $initMediaIdentifier = array_shift($args);
        // Gives the scheme (http, https)
        $scheme = array_shift($args);

        $reconstructedUrl = StreamingTechnologyHelper::reconstructUrlFromArray(
            $scheme,
            $args,
            $request->getQueryParams(),
        );

        return [
            "segmentType" => $segmentType,
            "initMediaIdentifier" => $initMediaIdentifier,
            "reconstructedUrl" => $reconstructedUrl,
            ];
    }
}
