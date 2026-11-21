VERSION := $(shell cat version.txt)

circumscribe:
	go -C src build -o ../circumscribe -ldflags "-X main.version=$(VERSION)"

.PHONY: circumscribe
