<?php

declare(strict_types=1);

namespace App\Controllers;

require_once __DIR__ . "../../../config/constants.php";

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
        return $this->repository->add($id, $content, DEFAULT_INIT_CONTENT_TTL);
    }

    public function getInitContent(string $id): string | null
    {
        return $this->repository->select($id);
    }

}
