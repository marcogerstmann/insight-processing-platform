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

REST_GOOS ?= linux
REST_GOARCH ?= amd64

WORKER_TAG ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo manual)
WORKER_REPO ?= $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com/$(PROJECT)-worker
WORKER_FUNCTION ?= $(PROJECT)-worker

.PHONY: test ingest-build rest-build worker-build worker-push tf-init tf-apply tf-destroy deploy tf-backend-bootstrap

# ============================================================
# General
# ============================================================

test:
	go test ./... -v

# ============================================================
# Terraform
# ============================================================

# The S3 backend uses an older AWS SDK that doesn't resolve the CLI's credential chain (SSO/profile helpers). Bridge the CLI-resolved creds into env vars it reads.
TF_AWS_CREDS = eval "$$(aws configure export-credentials --format env)"

tf-backend-bootstrap:
	bash terraform/scripts/bootstrap-backend.sh

tf-init:
	cd $(TF_DIR) && $(TF_AWS_CREDS) && terraform init

tf-apply: tf-init ingest-build rest-build
	cd $(TF_DIR) && $(TF_AWS_CREDS) && terraform apply -var="worker_image_uri=$(WORKER_REPO):$(WORKER_TAG)"

tf-destroy: tf-init
	cd $(TF_DIR) && $(TF_AWS_CREDS) && terraform destroy

# ============================================================
# Ingest Lambda
# ============================================================

ingest-build:
	cd cmd/ingest-lambda && \
	GOOS=$(INGEST_GOOS) GOARCH=$(INGEST_GOARCH) CGO_ENABLED=$(CGO_ENABLED) \
	go build -trimpath -ldflags="-s -w" -o bootstrap main.go

# ============================================================
# REST Lambda
# ============================================================

rest-build:
	cd cmd/rest-lambda && \
	GOOS=$(REST_GOOS) GOARCH=$(REST_GOARCH) CGO_ENABLED=$(CGO_ENABLED) \
	go build -trimpath -ldflags="-s -w" -o bootstrap main.go

# ============================================================
# Worker Lambda
# ============================================================

worker-build:
	docker buildx build --platform linux/amd64 \
		--provenance=false --sbom=false \
		--load \
		-t $(PROJECT)-worker:$(WORKER_TAG) \
		-f cmd/worker-lambda/Dockerfile .

worker-push:
	aws ecr get-login-password --region $(AWS_REGION) | docker login --username AWS --password-stdin $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com
	docker tag $(PROJECT)-worker:$(WORKER_TAG) $(WORKER_REPO):$(WORKER_TAG)
	docker push $(WORKER_REPO):$(WORKER_TAG)
	docker rmi $(PROJECT)-worker:$(WORKER_TAG) || true
	docker rmi $(WORKER_REPO):$(WORKER_TAG) || true

# ============================================================
# Deployment
# ============================================================

deploy: worker-build worker-push tf-apply
