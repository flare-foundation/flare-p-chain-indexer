# build executable
FROM golang:1.18 AS builder

WORKDIR /build

# Copy and download dependencies using go mod
COPY go.mod go.sum ./
RUN go mod download

# Copy the code into the container
COPY . ./

# Build the applications
RUN go build -o /app/flare_indexer ./indexer/main/indexer.go
# RUN go build -o /app/flare_services ./services/main/services.go

FROM debian:latest AS execution

ARG deployment=flare
ARG type=voting

RUN apt-get -y update && apt-get -y install curl

WORKDIR /app
COPY --from=builder /app/flare_indexer .
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY ./docker/indexer/config_${deployment}_${type}.toml ./config.toml

CMD ["./flare_indexer", "--config", "/app/config.toml" ]
