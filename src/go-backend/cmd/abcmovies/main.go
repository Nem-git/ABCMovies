package main

import (
	"log"

	"net/http"

	"github.com/nem-git/abcmovies/internal/handlers"
)

func main() {

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	mux := http.NewServeMux()

	handlers.Handler(mux)

	log.Println("Welcome to ABCMovies' Go API!")

	if err := http.ListenAndServe(":8090", mux); err != nil {
		log.Println(err)
	}
}
