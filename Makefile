.PHONY: generate test vet build run tidy clean

# Watch .templ files, regenerate, and proxy to the Go server with browser auto-reload.
live/proxy:
	go tool templ generate --watch --open-browser=false --proxy="http://127.0.0.1:80" --proxybind="0.0.0.0" -v

# run tailwindcss to generate the style.css bundle in watch mode.
live/tailwind:
	./tailwindcss -i ./internal/web/static/css/input.css -o ./internal/web/static/css/style.css --minify --watch=always

# run esbuild to generate the index.js bundle in watch mode.
live/esbuild:
	./esbuild ./internal/web/static/js/index.ts --bundle --outdir=./internal/web/static/js/ --minify --watch=forever

# Watch CSS output, send browser reload when tailwind finishes compiling.
live/sync:
	air -c ./config/air/static.toml

live/go:
	air -c ./config/air/go.toml

# start all watch processes in parallel.
live:
	make -j5 live/proxy live/sync live/tailwind live/esbuild live/go
generate/css:
	./tailwindcss -i ./internal/web/static/css/input.css -o ./internal/web/static/css/style.css --minify

# Unsure about what this index.ts is supposed to be for
generate/js:
	./esbuild ./internal/web/static/js/index.ts --bundle --outdir=./internal/web/static/js/ --minify

generate/templ:
	go tool templ generate

generate/go:
	go generate ./...

# generates oas code, css, js, templ
generate:
	make -j4 generate/css generate/js generate/templ generate/go

test:
	go test -v ./...

vet:
	go vet ./...

build:
	go build -o bin/abcmovies ./cmd/

run:
	go run ./cmd/ -config config.yaml

tidy:
	go mod tidy

clean:
	rm -rf bin/
