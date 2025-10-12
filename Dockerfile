FROM golang:1.25 AS builder
ARG BUILD_HASH=unknown
ARG CGO_ENABLED=0
ARG GOOS=linux
ARG GOARCH=amd64

ENV BUILD_HASH=${BUILD_HASH}
ENV CGO_ENABLED=${CGO_ENABLED}
ENV GOOS=${GOOS}
ENV GOARCH=${GOARCH}

WORKDIR /app

# Needs to install npm to be able to install the tailwind modules
RUN apt-get update && apt-get install -y npm

COPY . .

RUN go mod download
RUN npm ci

RUN npm run build
RUN go build -o ./bin/main cmd/main/main.go

FROM alpine:latest
WORKDIR /root/

# Copies binary from previous container
COPY --from=builder /app/bin/main .

EXPOSE 8080

CMD ["./main"]
