<?php

declare(strict_types=1);

namespace App\Helpers;

use Psr\Http\Message\ResponseInterface as Response;

class SlimResponseHelper
{
    public function response_json($data, Response $response): Response
    {
        $response->getBody()->write(json_encode($data, JSON_PRETTY_PRINT)); // TODO: Remove the pretty print, just for debug
        $response = $this->basic_response($response);
        $response = $response->withHeader("Content-Type", "application/json");
        return $response;
    }

    public function response_dash(string $data, Response $response): Response
    {
        $response->getBody()->write($data);
        $response = $this->basic_response($response);
        $response = $response->withHeader("Content-Type", "application/dash+xml");
        return $response;
    }

    public function response_segment($data, Response $response): Response
    {
        $response->getBody()->write($data);
        $response = $this->basic_response($response);
        $response = $response->withHeader("Content-Type", "video/mp4"); // TODO: WTF am I doing here, it depends on the content type ;)
        $response = $response->withHeader("Access-Control-Allow-Headers", "Range");
        return $response;
    }

    private function basic_response(Response $response): Response
    {
        $response = $response->withHeader("Access-Control-Allow-Origin", "*");
        $response = $response->withHeader("Content-Length", $response->getBody()->getSize());

        return $response;
    }
}
