module github.com/nem-git/abcmovies

go 1.27.0

toolchain go1.27.2

require (
	connectrpc.com/connect v1.21.0
	github.com/iyear/gowidevine v0.1.3
	go.yaml.in/yaml/v4 v4.0.0-rc.6
	golang.org/x/crypto v0.58.0
	golang.org/x/text v0.43.0
	google.golang.org/grpc v1.84.0
	google.golang.org/protobuf v1.36.12
	modernc.org/sqlite v1.60.1
)

require (
	github.com/Eyevinn/mp4ff v0.48.0 // indirect
	github.com/chmike/cmac-go v1.1.0 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/google/pprof v0.0.0-20261003200830-ebaad5f31b4d // indirect
	github.com/google/renameio v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.1.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/telemetry v0.0.0-20260924152758-ed294f943157 // indirect
	golang.org/x/tools v0.51.0 // indirect
	golang.org/x/vuln v1.8.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261005182115-fad411399dd8 // indirect
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.6.2 // indirect
	modernc.org/libc v1.78.0 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
)

tool (
	connectrpc.com/connect/cmd/protoc-gen-connect-go
	golang.org/x/vuln/cmd/govulncheck
	google.golang.org/grpc/cmd/protoc-gen-go-grpc
	google.golang.org/protobuf/cmd/protoc-gen-go
)
