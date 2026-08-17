.RECIPEPREFIX := >
.PHONY: fmt tidy verify list build test clean

fmt:
>go fmt ./...

tidy:
>go mod tidy

verify:
>go mod verify

list:
>go list -mod=readonly -m all

build:
>go build -trimpath -ldflags="-s -w" -o bin/goldbar ./cmd/goldbar

test:
>go test -timeout=120s -count=1 ./...

clean:
>rm -rf bin
