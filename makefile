SHELL := /bin/bash

-include .env
export

TF_ENV ?= dev
TF_DIR ?= terraform/envs/$(TF_ENV)

PROJECT ?= ipp-ingest

WORKER_IMAGE ?= $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com/$(PROJECT)-worker

CGO_ENABLED ?= 0

INGEST_GOOS ?= linux
INGEST_GOARCH ?= amd64

.PHONY: test ingest-build worker-build worker-push tf-init tf-apply tf-destroy worker-deploy

test:
	go test ./... -v

ingest-build:
	cd cmd/ingest-lambda && \
	GOOS=$(INGEST_GOOS) GOARCH=$(INGEST_GOARCH) CGO_ENABLED=$(CGO_ENABLED) \
	go build -trimpath -ldflags="-s -w" -o bootstrap main.go

worker-build:
	docker buildx build --platform linux/amd64 \
		-t $(PROJECT)-worker:latest \
		-f cmd/worker-lambda/Dockerfile .

worker-push:
	@if [ -z "$(AWS_REGION)" ] || [ -z "$(AWS_ACCOUNT_ID)" ]; then \
		echo "AWS_REGION and AWS_ACCOUNT_ID must be set"; exit 1; \
	fi
	docker tag $(PROJECT)-worker:latest $(WORKER_IMAGE):latest
	docker push $(WORKER_IMAGE):latest

tf-init:
	cd $(TF_DIR) && terraform init

tf-apply: tf-init ingest-build
	cd $(TF_DIR) && terraform apply

tf-destroy: tf-init
	cd $(TF_DIR) && terraform destroy

worker-deploy: worker-build worker-push tf-apply
