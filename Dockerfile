FROM golang:1.27-alpine AS builder

ENV GOOS=linux
ENV GOARCH=amd64

ARG VERSION=0.1.0

WORKDIR /redirectr

COPY go.mod ./go.mod
COPY go.sum ./go.sum
RUN go mod download

COPY . .
RUN go build -ldflags "-X main.Release=redirectr@${VERSION}" -o redirectr main.go
RUN go test ./...


FROM scratch

ENV API_PORT=8080
ENV HANDLER_PORT=8081
EXPOSE 8080
EXPOSE 8081

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /redirectr/redirectr /usr/bin/redirectr

ENTRYPOINT [ "/usr/bin/redirectr" ]