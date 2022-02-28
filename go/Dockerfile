# syntax=docker/dockerfile:1

# BUILD
FROM golang:1.17-buster AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY *.go ./

RUN go build -o /mux-test

# DEPLOY
FROM gcr.io/distroless/base-debian10

WORKDIR /

COPY --from=build /mux-test /mux-test

USER nonroot:nonroot

ENTRYPOINT [ "/mux-test" ]
