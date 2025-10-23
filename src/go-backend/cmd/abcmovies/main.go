package main

import (
	"log"

	"net/http"

	"github.com/nem-git/abcmovies/internal/http/route"
)

func main() {

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	mux := http.NewServeMux()

	route.Handler(mux)

	log.Println("Welcome to ABCMovies' Go API!")
	log.Println("http://localhost:8090/api/")

	if err := http.ListenAndServe(":8090", mux); err != nil {
		log.Println(err)
	}
}
