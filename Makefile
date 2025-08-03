.PHONY: build clean run
run: build
	./bin/pipeliner -s subdomain -d deepseek.com
build:
	go build -o bin/pipeliner ./cmd/pipeliner
clean:
	rm -rf bin