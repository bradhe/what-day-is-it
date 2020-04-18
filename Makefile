TARGET ?= what-day-is-it
BINDIR ?= ./bin

DOCKER_IMAGE_NAME ?= bradhe/$(TARGET)
DOCKER_IMAGE_TAG ?= latest

clean:
	rm -rf $(BINDIR)
	mkdir $(BINDIR)

build: clean
	go generate ./...
	go build -o $(BINDIR)/$(TARGET) ./cmd/what-day-is-it

test: clean
	go test -v ./...

images:
	docker build -t $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) .