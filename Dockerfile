# The main Build image to build our binaries
FROM golang:1.22.3-alpine3.19 as build

WORKDIR /

# Install UPX for binary compression
RUN apk add upx

# Go dependencies
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Copy source code
COPY ./cmd/api ./cmd/api

# build binary
WORKDIR /
RUN go build -ldflags="-s -w" -o api /cmd/api/api.go

# Create release image
FROM alpine:3.19 as release

# Copy our static executable
COPY --from=build /api /api

EXPOSE 8080
CMD [ "/api" ]
