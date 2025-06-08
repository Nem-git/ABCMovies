Contains repository classes that abstract away data storage and retrieval

Example:
// app/Repositories/PostRepository.php

namespace App\Repositories;

use App\Models\Post;
use PDO;

class PostRepository
{
    private $pdo;

    public function __construct(PDO $pdo)
    {
        $this->pdo = $pdo;
    }

    public function findAll()
    {
        $stmt = $this->pdo->query('SELECT * FROM posts');
        return $stmt->fetchAll(PDO::FETCH_CLASS, Post::class);
    }

    public function find($id)
    {
        $stmt = $this->pdo->prepare('SELECT * FROM posts WHERE id = :id');
        $stmt->execute([':id' => $id]);
        return $stmt->fetchObject(Post::class);
    }
}

├── Repositories/
│   ├── PostRepository.php
│   ├── UserRepository.php