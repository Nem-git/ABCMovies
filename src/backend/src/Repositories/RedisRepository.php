<?php

declare(strict_types=1);

namespace App\Repositories;

use Predis\Client as PredisClient;

class RedisRepository
{
    private PredisClient $conn;

    public function __construct()
    {
        $this->conn = new PredisClient();
    }

    public function select($key)
    {
        return $this->conn->get($key);
    }

    public function add(string $key, string $value, ?int $ttl)
    {
        return $this->conn->set($key, $value, expireTTL: $ttl);
    }

    public function addToList(string $key, array $value) // TODO: Add a TTL
    {
        return $this->conn->lpush($key, $value);
    }

    public function selectFromList($key, int $indexStart = 0, int $indexEnd = -1)
    {
        return $this->conn->lrange($key, $indexStart, $indexEnd);
    }

    public function deleteKey($key)
    {
        return $this->conn->del($key);
    }
}
