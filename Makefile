TEMPL_SRC_DIR = templates
TEMPL_FILES = $(shell find $(TEMPL_SRC_DIR) -name "*.templ")
.PHONY: build clean run generate-templ
run: build
	./bin/pipeliner -s subdomain -d deepseek.com
build: generate-templ
	go build -o bin/pipeliner ./cmd/pipeliner
clean:
	rm -rf bin

generate-templ: $(TEMPL_FILES)
	templ generate