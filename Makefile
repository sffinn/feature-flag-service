.PHONY: build run test clean

BIN := bin/feature-flag-service
CMD := ./cmd/feature-flag-service

build:
	mkdir -p bin
	go build -o $(BIN) $(CMD)

run: build
	$(BIN)

test:
	go test ./...

clean:
	rm -rf bin feature-flag-service server
