FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN go build -o /out/nodebridged ./cmd/nodebridged

FROM alpine:3.20
RUN adduser -D -H nodebridge
WORKDIR /app
COPY --from=build /out/nodebridged /usr/local/bin/nodebridged
COPY configs/nodebridge.example.json /etc/nodebridge/config.json
RUN mkdir -p /var/lib/nodebridge && chown -R nodebridge:nodebridge /var/lib/nodebridge
USER nodebridge
EXPOSE 8088
ENTRYPOINT ["/usr/local/bin/nodebridged"]
CMD ["-config", "/etc/nodebridge/config.json"]

