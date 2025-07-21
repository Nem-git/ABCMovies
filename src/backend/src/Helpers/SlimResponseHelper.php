<?php

declare(strict_types=1);

namespace App\Helpers;

use Psr\Http\Message\ResponseInterface as Response;

final class SlimResponseHelper
{
    public static function response_json(
        array|\App\Streaming\Classes\Show|\App\Streaming\Classes\Season|\App\Streaming\Classes\Episode $data,
        Response $response,
    ): Response {
        $response->getBody()->write(json_encode($data, JSON_PRETTY_PRINT));
        $response = self::basic_response($response);
        $response = $response->withHeader("Content-Type", "application/json");
        return $response;
    }

    public static function response_text($data, Response $response): Response
    {
        $response->getBody()->write($data);
        $response = self::basic_response($response);
        $response = $response->withHeader("Content-Type", "text/plain");
        return $response;
    }

    public static function response_dash(
        string $data,
        Response $response,
    ): Response {
        $response->getBody()->write($data);
        $response = self::basic_response($response);
        $response = $response->withHeader(
            "Content-Type",
            "application/dash+xml",
        );
        return $response;
    }

    public static function response_segment(
        string $data,
        Response $response,
    ): Response {
        $response->getBody()->write($data);
        $response = self::basic_response($response);
        $response = $response->withHeader("Content-Type", "video/mp4");
        $response = $response->withHeader(
            "Access-Control-Allow-Headers",
            "Range",
        );
        return $response;
    }

    private static function basic_response(Response $response): Response
    {
        $response = $response->withHeader("Access-Control-Allow-Origin", "*");
        $size = $response->getBody()->getSize();

        if (!is_null($size)) {
            $response = $response->withHeader("Content-Length", (string) $size);
        }

        return $response;
    }
}
