.PHONY: build test race vet fmt dev package

build:
	CGO_ENABLED=0 go build -trimpath -tags mygo_noinspector -o .build/Chaos ./cmd/chaos
	CGO_ENABLED=0 go build -trimpath -o .build/chaos-hosted ./cmd/chaos-hosted

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w cmd internal

dev:
	go run github.com/egoist/mygo/cmd/mygo dev

package:
	./package-dmg.sh
