FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download || true
COPY . .
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w \
              -X github.com/oglimmer/trivia/backend/internal/buildinfo.Version=${VERSION} \
              -X github.com/oglimmer/trivia/backend/internal/buildinfo.Commit=${GIT_COMMIT} \
              -X github.com/oglimmer/trivia/backend/internal/buildinfo.Time=${BUILD_TIME}" \
    -o /out/server ./cmd/server

FROM alpine:3.23
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/server /app/server
COPY migrations /app/migrations
EXPOSE 8080
ENTRYPOINT ["/app/server"]
