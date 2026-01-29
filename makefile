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

WORKER_TAG ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo manual)
WORKER_REPO ?= $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com/$(PROJECT)-worker
WORKER_FUNCTION ?= $(PROJECT)-worker

.PHONY: test ingest-build worker-build worker-push tf-init tf-apply tf-destroy worker-deploy

test:
	go test ./... -v
	
tf-init:
	cd $(TF_DIR) && terraform init

tf-apply: tf-init ingest-build
	cd $(TF_DIR) && terraform apply -var="worker_image_uri=$(WORKER_REPO):$(WORKER_TAG)"

tf-destroy: tf-init
	cd $(TF_DIR) && terraform destroy

ingest-build:
	cd cmd/ingest-lambda && \
	GOOS=$(INGEST_GOOS) GOARCH=$(INGEST_GOARCH) CGO_ENABLED=$(CGO_ENABLED) \
	go build -trimpath -ldflags="-s -w" -o bootstrap main.go

worker-build:
	docker buildx build --platform linux/amd64 \
		--load \
		-t $(PROJECT)-worker:$(WORKER_TAG) \
		-f cmd/worker-lambda/Dockerfile .

worker-push:
	aws ecr get-login-password --region $(AWS_REGION) | docker login --username AWS --password-stdin $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com
	docker tag $(PROJECT)-worker:$(WORKER_TAG) $(WORKER_REPO):$(WORKER_TAG)
	docker push $(WORKER_REPO):$(WORKER_TAG)
# 	Delete local docker image after push
	docker rmi $(PROJECT)-worker:$(WORKER_TAG) || true
	docker rmi $(WORKER_REPO):$(WORKER_TAG) || true

worker-deploy: worker-build worker-push tf-apply
