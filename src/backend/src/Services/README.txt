Contains service classes that encapsulate complex business logic

Example:
// app/Services/PostService.php

namespace App\Services;

use App\Repositories\PostRepository;

class PostService
{
    private $repository;

    public function __construct(PostRepository $repository)
    {
        $this->repository = $repository;
    }

    public function getPosts()
    {
        return $this->repository->findAll();
    }

    public function getPost($id)
    {
        return $this->repository->find($id);
    }
}

├── Services/
│   ├── PostService.php
│   ├── UserService.php