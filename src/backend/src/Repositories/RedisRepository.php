<?php

declare(strict_types=1);

namespace App\Repositories;

use Predis\Client as PredisClient;
use Predis\Response\Status;
use App\Config\Constants;

class RedisRepository
{
    private PredisClient $conn;

    public function __construct()
    {
        $this->conn = new PredisClient(
            [
            "host" => $_ENV["DB_HOST"] ?? null,
            "password" => $_ENV["DB_PW"] ?? null,
            "port" => $_ENV["DB_PORT"] ?? null,
            "database" => $_ENV["DB_ID"] ?? null,
            ]
        );
    }

    public function select(string $key): string|null
    {
        return $this->conn->get($key);
    }

    public function add($key, $value, ?int $ttl): Status|null
    {
        return $this->conn->set(
            $key,
            $value,
            Constants::DEFAULT_REDIS_TTL_TYPE,
            expireTTL: $ttl,
        );
    }

    public function addToList(string $key, array $value): int
    {
        return $this->conn->lpush($key, $value);
    }

    public function selectFromList(
        string $key,
        int $indexStart = 0,
        int $indexEnd = -1,
    ): array {
        return $this->conn->lrange($key, $indexStart, $indexEnd);
    }

    public function deleteKey($key): int
    {
        return $this->conn->del($key);
    }
}
