<?php

declare(strict_types=1);

namespace App\Streaming\StreamingTechnology\Dash\Helpers;

use Psr\Http\Message\ServerRequestInterface as Request;
use App\Streaming\StreamingTechnology\Helpers\StreamingTechnologyHelper;
use App\Streaming\Helpers\RequestHelper;

class DashHelper
{
    public static function parseDashSegmentCriteria(Request $request, array $args): array
    {
        // Gives the adaptation set unique ID
        $initMediaIdentifier = array_shift($args);
        // Gives the scheme (http, https)
        $scheme = array_shift($args);

        // Add the query params to the last part of the URL
        $args[count($args) - 1] .= RequestHelper::format_parameters($request->getQueryParams() ?? []);

        $reconstructedUrl = StreamingTechnologyHelper::reconstructUrlFromArray($scheme, $args);

        return [
            "initMediaIdentifier" => $initMediaIdentifier,
            "reconstructedUrl" => $reconstructedUrl,
        ];
    }
}
