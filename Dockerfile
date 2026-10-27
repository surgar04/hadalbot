FROM golang:1.27.3-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILT_AT=unknown
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/surgar04/hadalbot/internal/version.Version=${VERSION} -X github.com/surgar04/hadalbot/internal/version.Commit=${COMMIT} -X github.com/surgar04/hadalbot/internal/version.BuiltAt=${BUILT_AT}" -o /out/hadalbot ./cmd/hadalbot \
 && CGO_ENABLED=0 go build -trimpath -o /out/migrate ./cmd/migrate

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && addgroup -S -g 10001 hadalbot && adduser -S -D -H -u 10001 -G hadalbot hadalbot
WORKDIR /app
COPY --from=build /out/ /app/
COPY configs/production.yaml /app/configs/production.yaml
COPY LICENSE THIRD_PARTY_NOTICES.md /app/
COPY licenses/ /app/licenses/
ENV HADALBOT_ENVIRONMENT=production HADALBOT_HTTP_ADDRESS=0.0.0.0:18878
USER 10001:10001
EXPOSE 18878
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s CMD ["/app/hadalbot", "-healthcheck"]
ENTRYPOINT ["/app/hadalbot"]
CMD ["-config", "/app/configs/production.yaml"]
