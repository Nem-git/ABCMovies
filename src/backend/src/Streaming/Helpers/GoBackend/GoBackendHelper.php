<?php

declare(strict_types=1);

namespace App\Streaming\Helpers\GoBackend;

use App\Streaming\Helpers\RequestHelper;
use App\Factory\ObjectFactory;
use App\Streaming\Helpers\GoBackend\Classes\GoBackendResponse;

final class GoBackendHelper
{
    public static function get(string $endpoint, array $data): GoBackendResponse
    {
        $response = json_decode(
            RequestHelper::post(
                $_ENV["GO_BACKEND_URL"] . $endpoint,
                data: $data,
            ),
            true,
        );

        return ObjectFactory::createGoBackendResponse($response);
    }
}
