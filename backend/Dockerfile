FROM golang:1.27.3-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /tracker ./cmd/tracker
# Intake/Nostr container. Research needs a host service with Docker + Node + Git;
# never mount the host Docker socket into an untrusted research container.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /tracker /tracker
WORKDIR /data
ENV HTTP_ADDR=0.0.0.0:8080 DATABASE_PATH=/data/tracker.sqlite
EXPOSE 8080
ENTRYPOINT ["/tracker"]
