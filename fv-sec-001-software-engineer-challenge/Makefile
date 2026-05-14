BINARY := aggregator
INPUT  := ./ad_data.csv
OUTPUT := ./results
TOP    := 10

.PHONY: build test run bench fmt vet clean

build:
	go build -trimpath -ldflags="-s -w" -o $(BINARY) ./cmd/aggregator

test:
	go test ./...

run: build
	./$(BINARY) --input $(INPUT) --output $(OUTPUT) --top $(TOP)

# Wall-clock timing plus peak resident memory, sampled from /proc.
bench: build
	@./$(BINARY) --input $(INPUT) --output $(OUTPUT) --top $(TOP) & \
	pid=$$!; peak=0; \
	while kill -0 $$pid 2>/dev/null; do \
		hwm=$$(awk '/VmHWM/{print $$2}' /proc/$$pid/status 2>/dev/null); \
		[ -n "$$hwm" ] && [ "$$hwm" -gt "$$peak" ] && peak=$$hwm; \
		sleep 0.05; \
	done; wait $$pid; \
	echo "peak RSS: $$peak kB (~$$((peak/1024)) MiB)"

fmt:
	gofmt -w .

vet:
	go vet ./...

clean:
	rm -f $(BINARY)
