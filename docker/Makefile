#Makefile related to docker

DOCKER_IMAGE ?= unfoldingword/dcs
DOCKER_TAG ?= latest
DOCKER_REF := $(DOCKER_IMAGE):$(DOCKER_TAG)

.PHONY: docker
docker:
	docker build --disable-content-trust=false -t $(DOCKER_REF) .
# support also build args docker build --build-arg GITEA_VERSION=v1.2.3 --build-arg TAGS="bindata sqlite sqlite_unlock_notify"  .

.PHONY: docker-build
docker-build:
	docker run -ti --rm -v $(CURDIR):/srv/app/src/code.gitea.io/gitea -w /srv/app/src/code.gitea.io/gitea -e TAGS="bindata $(TAGS)" LDFLAGS="$(LDFLAGS)" CGO_EXTRA_CFLAGS="$(CGO_EXTRA_CFLAGS)" webhippie/golang:edge make clean build
