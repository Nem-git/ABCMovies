<?php

declare(strict_types=1);

namespace App\Controllers;

use App\Config\Constants;

/**
 * The name is so wrong in so many ways.
 * This has nothing to do with manifest
 * specifically and more to do with DRM
 */
class ManifestController
{
    private $repository; // Database Repository

    public function __construct($repository)
    {
        $this->repository = $repository;
    }

    public function addDecryptionKeys(string $id, array $decryptionKeys): int
    {
        $this->repository->deleteKey($id);
        return $this->repository->addToList($id, $decryptionKeys);
    }

    public function getDecryptionKeys(string $id): array
    {
        return $this->repository->selectFromList($id);
    }

    public function addInitContent(string $id, string $content)
    {
        return $this->repository->add(
            $id,
            $content,
            Constants::DEFAULT_INIT_CONTENT_TTL,
        );
    }

    public function getInitContent(string $id): string|null
    {
        return $this->repository->select($id);
    }
}
