package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/handler"
	"github.com/nem-git/abcmovies/internal/middleware"
	"github.com/nem-git/abcmovies/internal/oas"
	"github.com/nem-git/abcmovies/internal/providers"
	"github.com/nem-git/abcmovies/internal/registry"
	"github.com/nem-git/abcmovies/internal/web"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to configuration file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	r := registry.New()

	for _, entry := range cfg.Services {
		p, err := providers.Build(entry, cfg.Server.BaseURL, cfg.Server.APIPrefix)
		if err != nil {
			log.Fatalf("building provider %q: %v", entry.Tag, err)
		}
		if err := r.Register(p); err != nil {
			log.Fatalf("registering provider %q: %v", entry.Tag, err)
		}
		log.Printf("registered provider: %s (%s)", entry.Tag, entry.Type)
	}

	webHandler := web.New(r, cfg.Server.BaseURL, cfg.Server.APIPrefix)

	h := handler.New(r)

	srv, err := oas.NewServer(
		h,
		oas.WithErrorHandler(handler.ErrorHandler),
		oas.WithPathPrefix(cfg.Server.APIPrefix),
	)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle(cfg.Server.APIPrefix+"/", middleware.ApiSecurity(srv))
	mux.Handle("/", middleware.FrontendSecurity(webHandler))

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
