# Container image configuration
IMG_REGISTRY ?= ${{values.image.registry}}
IMG_NAMESPACE ?= ${{values.image.namespace}}
IMG_REPO ?= ${{values.image.repoName}}
IMG_TAG ?= latest
IMG = $(IMG_REGISTRY)/$(IMG_NAMESPACE)/$(IMG_REPO):$(IMG_TAG)

.PHONY: generate
generate:
	go generate ./...
	$(MAKE) catalog-api

.PHONY: fmt
fmt:
	go tool golang.org/x/tools/cmd/goimports -w .

.PHONY: lint
lint:
	go tool github.com/golangci/golangci-lint/cmd/golangci-lint run

.PHONY: lint-fix
lint-fix:
	go tool github.com/golangci/golangci-lint/cmd/golangci-lint run --fix

.PHONY: test
test:
	go test ./...

.PHONY: build
build:
	go build -o bin/app .

.PHONY: docker-build
docker-build:
	docker build -t $(IMG) .

.PHONY: trivy-scan
trivy-scan:
	docker run --rm --name trivy-scanner \
		-v /var/run/docker.sock:/var/run/docker.sock \
		aquasec/trivy image \
		--severity HIGH,CRITICAL \
		$(IMG)

.PHONY: run
run: build
	./bin/app

.PHONY: dev
dev:
	go run .

.PHONY: catalog-api
catalog-api:
	@sed '/    REPLACE/d' catalog-api.base.yaml > catalog-api.yaml
	@sed 's/^/    /' openapi.yaml >> catalog-api.yaml

.PHONY: clean
clean:
	rm -rf bin/
