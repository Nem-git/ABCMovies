Contains model classes that represent data and business logic

Example:
// app/Models/Post.php

namespace App\Models;

class Post
{
    private $id;
    private $title;
    private $content;

    public function __construct($id, $title, $content)
    {
        $this->id = $id;
        $this->title = $title;
        $this->content = $content;
    }

    public function getTitle()
    {
        return $this->title;
    }

    public function getContent()
    {
        return $this->content;
    }
}

├── Models/
│   ├── Post.php
│   ├── User.php