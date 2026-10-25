APP     := yonime.exe
VERSION ?=
REPO    ?= arfush/yonime
ARCH    ?= amd64

.DEFAULT_GOAL := build

.PHONY: build installer run test vet fmt check clean help

build:
	powershell -NoProfile -ExecutionPolicy Bypass -File .\build.ps1 -Version "$(VERSION)" -Repo "$(REPO)" -Arch "$(ARCH)"

installer: build
	powershell -NoProfile -ExecutionPolicy Bypass -File .\installer.ps1 -Version "$(VERSION)" -Arch "$(ARCH)"

run: build
	cmd /c start "" "$(APP)"

test:
	go test ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

check: vet test

clean:
	cmd /c "if exist $(APP) del /Q $(APP)"
	cmd /c "if exist dist rmdir /S /Q dist"

help:
	@echo Usage: mingw32-make [target] [VERSION=x.y] [REPO=owner/repo] [ARCH=amd64^|arm64^|386]
	@echo Targets: build installer run test vet fmt check clean help
	@echo Example: mingw32-make build VERSION=1.2 ARCH=amd64
