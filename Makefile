# ******************************************************************************
# IBM Cloud Kubernetes Service, 5737-D43
# (C) Copyright IBM Corp. 2021 All Rights Reserved.
#
# SPDX-License-Identifier: Apache2.0
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# ******************************************************************************
export
BINARY_NAME:=vpcctl
GO_PACKAGES:=$(shell go list ./...)
GOLANGCI_LINT_EXISTS:=$(shell golangci-lint --version 2>/dev/null)
GOLANGCI_LINT_VERSION?=1.40.0
INSTALL_LOCATION:=$(shell go env GOPATH)/bin
TRAVIS_BUILD_NUMBER?=$(shell date "+%Y-%m-%d")

.PHONY: all
all: lint vet test

.PHONY: build
build: build-darwin build-linux

.PHONY: build-darwin
build-darwin:
	@rm -f ${BINARY_NAME}-darwin
	cd cmd; CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -X main.version=${TRAVIS_BUILD_NUMBER}" -o ../${BINARY_NAME}-darwin .

.PHONY: build-linux
build-linux:
	@rm -f ${BINARY_NAME}-linux
	cd cmd; CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X main.version=${TRAVIS_BUILD_NUMBER}" -o ../${BINARY_NAME}-linux .

.PHONY: clean
clean:
	@rm -f ${BINARY_NAME}
	@rm -f ${BINARY_NAME}-darwin
	@rm -f ${BINARY_NAME}-linux

.PHONY: deps
deps:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh --fail | sh -s -- -b $(INSTALL_LOCATION) v$(GOLANGCI_LINT_VERSION)

.PHONY: fmt
fmt:
ifdef GOLANGCI_LINT_EXISTS
	golangci-lint run -v --disable-all --enable=gofmt --fix
else
	@echo "golangci-lint is not installed"
endif

.PHONY: lint
lint:
ifdef GOLANGCI_LINT_EXISTS
	golangci-lint run -v --timeout 5m
else
	@echo "golangci-lint is not installed"
endif

.PHONY: local
local:
	cd cmd; CGO_ENABLED=0 GOARCH=amd64 go build -ldflags "-s -w -X main.version=${TRAVIS_BUILD_NUMBER}" -o ../${BINARY_NAME} .

.PHONY: oss
oss:
	@test -f "LICENSE" || (echo "LICENSE file does not exist" && exit 1)

.PHONY: test
test:
	go test -race -covermode=atomic ${GO_PACKAGES}

.PHONY: vet
vet:
	go vet ${GO_PACKAGES}
