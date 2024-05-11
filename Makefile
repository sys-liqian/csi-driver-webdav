GIT_COMMIT = $(shell git rev-parse HEAD)
BUILD_DATE = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

PKG = github.com/sys-liqian/csi-driver-webdav
LDFLAGS = -X ${PKG}/pkg/webdav.driverVersion=${IMAGE_VERSION} -X ${PKG}/pkg/webdav.gitCommit=${GIT_COMMIT} -X ${PKG}/pkg/webdav.buildDate=${BUILD_DATE}
EXT_LDFLAGS = -s -w -extldflags "-static"

IMAGE_VERSION ?= v0.0.1
LOCAL_REPOSITORY ?= localhost:5000

.PHONY: go-build
go-build:
	CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) go build -a -ldflags "${LDFLAGS} ${EXT_LDFLAGS}" -o bin/webdavplugin ./cmd/webdav

.PHONY: docker-build
docker-build: go-build
	docker build --network host -t $(LOCAL_REPOSITORY)/webdavplugin:$(IMAGE_VERSION) .