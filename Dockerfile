FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/dge-api ./cmd/dge-api

FROM alpine:3.21
# ghostscript powers print-ready post-processing (CMYK/PDF-X) — MPE-56;
# its bundled iccprofiles/ dir provides the default OutputIntent profile
RUN apk add --no-cache ghostscript && adduser -D -u 10001 app
USER app
COPY --from=build /bin/dge-api /bin/dge-api
EXPOSE 8090
ENTRYPOINT ["/bin/dge-api"]
