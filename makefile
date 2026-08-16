SHELL := /bin/bash

-include .env
export

TF_ENV ?= dev
TF_DIR ?= terraform/envs/$(TF_ENV)

PROJECT ?= ipp-dev

WORKER_IMAGE ?= $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com/$(PROJECT)-worker

CGO_ENABLED ?= 0

READWISE_GOOS ?= linux
READWISE_GOARCH ?= amd64

REST_GOOS ?= linux
REST_GOARCH ?= amd64

RAINDROP_POLL_GOOS ?= linux
RAINDROP_POLL_GOARCH ?= amd64

WORKER_TAG ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo manual)
WORKER_REPO ?= $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com/$(PROJECT)-worker
WORKER_FUNCTION ?= $(PROJECT)-worker

AI_TAG ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo manual)
AI_REPO ?= $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com/$(PROJECT)-ai

.PHONY: test lint readwise-build rest-build raindrop-poll-build worker-build worker-push tf-init tf-apply tf-destroy deploy tf-backend-bootstrap ai-test ai-lint ai-run-local ai-build ai-push

# ============================================================
# General
# ============================================================

test:
	go test ./... -v

lint:
	golangci-lint run ./...

# ============================================================
# AI service (Python)
# ============================================================

ai-test:
	cd services/ai && uv run pytest

ai-lint:
	cd services/ai && uv run ruff check . && uv run ruff format --check .

ai-run-local:
	cd services/ai && uv run python -m ipp_ai

ai-build:
	docker buildx build --platform linux/amd64 \
		--provenance=false --sbom=false \
		--load \
		-t $(PROJECT)-ai:$(AI_TAG) \
		-f services/ai/Dockerfile services/ai

ai-push:
	aws ecr get-login-password --region $(AWS_REGION) | docker login --username AWS --password-stdin $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com
	docker tag $(PROJECT)-ai:$(AI_TAG) $(AI_REPO):$(AI_TAG)
	docker push $(AI_REPO):$(AI_TAG)
	docker rmi $(PROJECT)-ai:$(AI_TAG) || true
	docker rmi $(AI_REPO):$(AI_TAG) || true

# ============================================================
# Terraform
# ============================================================

# The S3 backend uses an older AWS SDK that doesn't resolve the CLI's credential chain (SSO/profile helpers). Bridge the CLI-resolved creds into env vars it reads.
TF_AWS_CREDS = eval "$$(aws configure export-credentials --format env)"

tf-backend-bootstrap:
	bash terraform/scripts/bootstrap-backend.sh

tf-init:
	cd $(TF_DIR) && $(TF_AWS_CREDS) && terraform init

tf-apply: tf-init readwise-build rest-build raindrop-poll-build
	cd $(TF_DIR) && $(TF_AWS_CREDS) && terraform apply \
		-var="worker_image_uri=$(WORKER_REPO):$(WORKER_TAG)" \
		-var="ai_image_uri=$(AI_REPO):$(AI_TAG)"

tf-destroy: tf-init
	cd $(TF_DIR) && $(TF_AWS_CREDS) && terraform destroy

# ============================================================
# Readwise Lambda
# ============================================================

readwise-build:
	cd cmd/readwise-lambda && \
	GOOS=$(READWISE_GOOS) GOARCH=$(READWISE_GOARCH) CGO_ENABLED=$(CGO_ENABLED) \
	go build -trimpath -ldflags="-s -w" -o bootstrap main.go

# ============================================================
# REST Lambda
# ============================================================

rest-build:
	cd cmd/rest-lambda && \
	GOOS=$(REST_GOOS) GOARCH=$(REST_GOARCH) CGO_ENABLED=$(CGO_ENABLED) \
	go build -trimpath -ldflags="-s -w" -o bootstrap main.go

# ============================================================
# Raindrop Poll Lambda
# ============================================================

raindrop-poll-build:
	cd cmd/raindrop-poll-lambda && \
	GOOS=$(RAINDROP_POLL_GOOS) GOARCH=$(RAINDROP_POLL_GOARCH) CGO_ENABLED=$(CGO_ENABLED) \
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

deploy: worker-build worker-push ai-build ai-push tf-apply
