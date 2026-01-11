SHELL := /bin/bash

LAMBDA_DIR := ingress-lambda
TF_DIR := infra
BOOTSTRAP := $(LAMBDA_DIR)/bootstrap

GOOS ?= linux
GOARCH ?= amd64
CGO_ENABLED ?= 0

.PHONY: build-lambda-ingress tf-init tf-apply tf-destroy deploy

build-lambda-ingress:
	cd $(LAMBDA_DIR) && \
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=$(CGO_ENABLED) \
	go build -o bootstrap main.go

tf-init:
	cd $(TF_DIR) && terraform init

tf-apply:
	cd $(TF_DIR) && terraform apply

tf-destroy:
	cd $(TF_DIR) && terraform destroy

deploy: build-lambda-ingress tf-apply
