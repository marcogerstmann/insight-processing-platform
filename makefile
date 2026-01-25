SHELL := /bin/bash

LAMBDA_DIR := cmd/ingest-lambda
TF_ENV ?= dev
TF_DIR ?= terraform/envs/$(TF_ENV)
BOOTSTRAP := $(LAMBDA_DIR)/bootstrap

GOOS ?= linux
GOARCH ?= arm64
CGO_ENABLED ?= 0

.PHONY: build-lambda-ingest tf-init tf-apply tf-destroy deploy

test:
	go test ./... -v

build-lambda-ingest:
	cd $(LAMBDA_DIR) && \
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=$(CGO_ENABLED) \
	go build -trimpath -ldflags="-s -w" -o bootstrap main.go

tf-init:
	cd $(TF_DIR) && terraform init

tf-apply: build-lambda-ingest
	cd $(TF_DIR) && terraform apply

tf-destroy:
	cd $(TF_DIR) && terraform destroy
