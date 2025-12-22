FROM golang:1.25-alpine AS base

WORKDIR /srv/www

RUN apk add --no-cache make
RUN apk add g++

COPY go.mod go.sum ./

RUN go mod download

COPY Makefile ./
COPY bin/ ./bin
COPY cmd/ ./cmd
COPY internal/ ./internal
COPY plugins/ ./plugins

RUN make

FROM alpine:latest AS go-backend-dev

COPY --from=base /srv/www/build ./build

CMD [ "./build/abcmovies.out" ]

FROM alpine:latest AS go-backend-prod

COPY --from=base /srv/www/build ./build

CMD [ "./build/abcmovies.out" ]
