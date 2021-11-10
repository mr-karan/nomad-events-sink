FROM ubuntu:20.04
WORKDIR /app
COPY nomad-events-sink.bin .
COPY config.sample.toml .
CMD ["./nomad-events-sink.bin"]
