FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /sitemap-audit ./cmd/sitemap-audit

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /sitemap-audit /usr/local/bin/sitemap-audit
WORKDIR /work
ENTRYPOINT ["/usr/local/bin/sitemap-audit"]
CMD ["help"]
