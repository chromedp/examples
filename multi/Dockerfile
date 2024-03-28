FROM golang:latest AS builder
WORKDIR /app
COPY go.mod go.sum /app/
COPY multi/main.go /app
RUN \
  go build -v -x -o multi \
  && du -sh multi

FROM docker.io/chromedp/headless-shell:latest
COPY \
  --from=builder \
  /app/multi /app/multi
WORKDIR /app

ENTRYPOINT [ "/app/multi", "-v", "-out", "/out" ]
