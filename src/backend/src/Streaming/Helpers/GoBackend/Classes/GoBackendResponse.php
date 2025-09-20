<?php

declare(strict_types=1);

namespace App\Streaming\Helpers\GoBackend\Classes;

final class GoBackendResponse
{
    public string $error;
    public string $pssh;
    public array $keys;
    public string $content;
    public string $segment;

    public function __construct(array $data)
    {
        foreach ($data as $key => $value) {
            $this->$key = $value;
        }
    }
}
