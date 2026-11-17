.PHONY: build test run-mock sim digest-sample fmt

build:
	go build -o bin/roger-relay ./cmd/roger-relay
	go build -o bin/roger-sim ./cmd/roger-sim

test:
	go vet ./...
	go test ./... -race -count=1

# No keys needed: every press is answered with a chirp and an echo.
run-mock: build
	ROGER_PROVIDER=mock ./bin/roger-relay

# One simulated press against a running relay. Needs ROGER_TOKEN.
sim: build
	./bin/roger-sim -out reply.wav

# Re-render the sample digest that lives in parent/digest/.
digest-sample:
	go test ./internal/digest -run TestWriteSample -count=1 -sample ../../../parent/digest/sample.html

fmt:
	gofmt -w .
