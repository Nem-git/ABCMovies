<?php

declare(strict_types=1);

namespace App\Helpers;

use Psr\Http\Message\ResponseInterface as Response;

class SlimResponseHelper
{
    public function response_json($data, Response $response): Response
    {
        $response = $response->withHeader("Content-Type", "application/json");
        $response->getBody()->write(json_encode($data, JSON_PRETTY_PRINT)); // TODO: Remove the pretty print, just for debug
        return $response;
    }

    public function response_dash(string $data, Response $response): Response
    {
        $response = $response->withHeader("Content-Type", "application/dash+xml");
        $response->getBody()->write($data);
        return $response;
    }
}
