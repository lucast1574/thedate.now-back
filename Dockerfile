FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /thedate-api ./cmd/api

FROM alpine:3.22
RUN adduser -D -H app
USER app
COPY --from=build /thedate-api /usr/local/bin/thedate-api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/thedate-api"]

