# --- build stage ---
FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bin/aggregator ./cmd/aggregator

# --- runtime stage ---
FROM gcr.io/distroless/static-debian12
COPY --from=build /bin/aggregator /aggregator
# Mount the data and a results directory at run time, e.g.:
#   docker run --rm -v "$PWD:/data" -w /data adperf \
#     --input /data/ad_data.csv --output /data/results --top 10
ENTRYPOINT ["/aggregator"]
