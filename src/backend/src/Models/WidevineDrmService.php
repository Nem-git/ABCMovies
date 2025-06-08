<?php

declare(strict_types=1);

namespace App\Models;

abstract class WidevineDrmService
{
    abstract public function get_pssh(DownloadInfo $downloadInfo): void;
    abstract public function get_decryption_keys(DownloadInfo $downloadInfo): void;
}
