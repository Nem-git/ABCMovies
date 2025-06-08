Contains controller classes that handle incoming requests.

Example:
// app/Controllers/PostController.php

namespace App\Controllers;

use App\Services\PostService;

class PostController
{
    private $service;

    public function __construct(PostService $service)
    {
        $this->service = $service;
    }

    public function index()
    {
        $posts = $this->service->getPosts();
        // Render the view with the posts
    }

    public function show($id)
    {
        $post = $this->service->getPost($id);
        // Render the view with the post
    }
}

├── Controllers/
│   ├── PostController.php
│   ├── UserController.php