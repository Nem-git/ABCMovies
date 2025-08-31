<?php

declare(strict_types=1);

namespace App\Streaming\Helpers\PythonBackend\Classes;

final class PythonBackendResponse
{
    public string $error;
    public string $pssh;
    public array $keys;
    public string $content;

    public function __construct(array $data)
    {
        foreach ($data as $key => $value) {
            $this->$key = $value;
        }
    }
}
