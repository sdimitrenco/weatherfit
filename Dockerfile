FROM golang:1.26-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates && mkdir -p /data

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/weatherbot ./cmd/weatherbot

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/weatherbot /usr/local/bin/weatherbot
COPY --from=builder --chown=nonroot:nonroot /data /data

USER nonroot:nonroot
ENV DB_PATH=/data/weatherfit.db

ENTRYPOINT ["/usr/local/bin/weatherbot"]
