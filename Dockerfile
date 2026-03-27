FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY archivarr.go .
RUN go build -o archivarr archivarr.go

FROM alpine:latest
COPY --from=builder /app/archivarr /archivarr
ENTRYPOINT ["/archivarr"]
