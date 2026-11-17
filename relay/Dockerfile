# Build
FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/roger-relay ./cmd/roger-relay

# Run: no shell, no package manager, non-root
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/roger-relay /roger-relay
WORKDIR /roger
ENV ROGER_ADDR=:8080 ROGER_DATA_DIR=/roger/data ROGER_DEVICES=/roger/devices.json
EXPOSE 8080
ENTRYPOINT ["/roger-relay"]
CMD ["serve"]
