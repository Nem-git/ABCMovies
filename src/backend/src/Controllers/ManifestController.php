<?php

declare(strict_types=1);

namespace App\Controllers;

class ManifestController
{
    private $repository; // Database Repository

    public function __construct($repository)
    {
        $this->repository = $repository;
    }

    public function addDecryptionKeys($id, array $decryptionKeys)
    {
        $this->repository->deleteKey($id);
        return $this->repository->addToList($id, $decryptionKeys);
    }

    public function getDecryptionKeys($id): array
    {
        return $this->repository->selectFromList($id);
    }

    public function addInitContent($id, string $content)
    {
        return $this->repository->add($id, $content, null);
    }

    public function getInitContent($id)
    {
        return $this->repository->select($id);
    }

}
