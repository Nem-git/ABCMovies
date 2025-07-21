<?php

declare(strict_types=1);

namespace App\Streaming\Helpers\PythonBackend;

use App\Streaming\Helpers\RequestHelper;
use App\Streaming\Helpers\PythonBackend\Classes\PythonBackendResponse;
use App\Factory\ObjectFactory;

final class PythonBackendHelper
{
    public static function get(
        string $endpoint,
        array $data,
    ): PythonBackendResponse {
        $response = json_decode(
            RequestHelper::post(
                $_ENV["PYTHON_BACKEND_URL"] . $endpoint,
                data: $data,
            ),
            true,
        );

        return ObjectFactory::createPythonBackendResponse($response);
    }
}
