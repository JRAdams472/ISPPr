FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/dge-api ./cmd/dge-api

FROM alpine:3.21
RUN adduser -D -u 10001 app
USER app
COPY --from=build /bin/dge-api /bin/dge-api
EXPOSE 8090
ENTRYPOINT ["/bin/dge-api"]
