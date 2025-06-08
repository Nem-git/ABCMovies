<?php

declare(strict_types=1);

namespace App\Models;

abstract class WidevineDrmService
{
    abstract public function get_pssh(string $mpdUrl, array $mpdHeaders = [], array $segmentsHeaders = []): string;
    abstract public function get_decryption_keys(string $pssh, string $licenseUrl, array $licenseHeaders = []): array;
}
