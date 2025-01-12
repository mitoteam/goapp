# ------ USAGE EXAMPLE -------
# EXECUTABLE_NAME := exec_name
# SUBMODULES := ./internal/dhtml
# ARCH_FILES := ./VERSION ./LICENSE.md # files to add to archives
# SKIP_EXECUTABLE_REMOVAL := 1 # do not delete executable files after packing them to 7z
#
# include internal/goapp/Makefile.inc.mk
# ----------------------------


# SET DEFAULT VALUES
# name of executable file without extension
ifeq (${EXECUTABLE_NAME},)
  EXECUTABLE_NAME := executable_name_not_set
endif

# app name
ifeq (${APP_NAME},)
  APP_NAME := ${EXECUTABLE_NAME}
endif

# directory to create distribution archives
ifeq (${DIST_DIR},)
  DIST_DIR := dist
endif

# internal submodules base path
ifeq (${INTERNAL_SUBMODULES_PATH},)
  INTERNAL_SUBMODULES_PATH := ./internal
endif

# submodules to run tests for
SUBMODULES := ${INTERNAL_SUBMODULES_PATH}/goapp ${SUBMODULES}

#set default target
.DEFAULT_GOAL := build-all


ifeq (${OS},Windows_NT)
	BUILD_TIME := $(shell powershell "Get-Date -Format 'yyyy-MM-dd HH:mm:ss'")
else
	BUILD_TIME := $(shell date +"%Y-%m-%d %H:%M:%S")
endif

APP_VERSION := $(file < VERSION)
APP_COMMIT := $(shell git rev-list -1 HEAD)
# see https://stackoverflow.com/a/22276273/380123 for -w -s
LD_FLAGS := "-w -s -X 'github.com/mitoteam/goapp.BuildVersion=${APP_VERSION}' -X 'github.com/mitoteam/goapp.BuildCommit=${APP_COMMIT}' -X 'github.com/mitoteam/goapp.BuildTime=${BUILD_TIME}'"


# all platforms build
.PHONY: build-all
build-all:: before-build dist-linux64 dist-linux32 dist-windows64 dist-windows32 after-build


.PHONY: build-windows32
build-windows32: before-build dist-windows32 after-build

.PHONY: dist-windows32
dist-windows32:
	GOOS=windows GOARCH=386 go build -o ${DIST_DIR}/${EXECUTABLE_NAME}.exe -ldflags=${LD_FLAGS} main.go
	7z a ${DIST_DIR}/${APP_NAME}-${APP_VERSION}-win32.7z -mx9 ./${DIST_DIR}/${EXECUTABLE_NAME}.exe ${ARCH_FILES}


.PHONY: build-windows64
build-windows64: before-build dist-windows64 after-build

.PHONY: dist-windows64
dist-windows64:
	GOOS=windows GOARCH=amd64 go build -o ${DIST_DIR}/${EXECUTABLE_NAME}.exe -ldflags=${LD_FLAGS} main.go
	7z a ${DIST_DIR}/${APP_NAME}-${APP_VERSION}-win64.7z -mx9 ./${DIST_DIR}/${EXECUTABLE_NAME}.exe ${ARCH_FILES}


.PHONY: build-linux32
build-linux32: before-build dist-linux32 after-build

.PHONY: dist-linux32
dist-linux32:
	GOOS=linux GOARCH=386 go build -o ${DIST_DIR}/${EXECUTABLE_NAME} -ldflags=${LD_FLAGS} main.go
	7z a ${DIST_DIR}/${APP_NAME}-${APP_VERSION}-linux32.7z -mx9 ./${DIST_DIR}/${EXECUTABLE_NAME} ${ARCH_FILES}


.PHONY: build-linux64
build-linux64: before-build dist-linux64 after-build

.PHONY: dist-linux64
dist-linux64:
	GOOS=linux GOARCH=amd64 go build -o ${DIST_DIR}/${EXECUTABLE_NAME} -ldflags=${LD_FLAGS} main.go
	7z a ${DIST_DIR}/${APP_NAME}-${APP_VERSION}-linux64.7z -mx9 ./${DIST_DIR}/${EXECUTABLE_NAME} ${ARCH_FILES}


.PHONY: before-build
before-build:: clean tests ${DIST_DIR}


.PHONY: after-build
after-build::
  ifeq (${SKIP_EXECUTABLE_REMOVAL},)
	  rm -f ${DIST_DIR}/${EXECUTABLE_NAME}
	  rm -f ${DIST_DIR}/${EXECUTABLE_NAME}.exe
  endif
	sha256sum ${DIST_DIR}/*.7z > ${DIST_DIR}/checksums.sha256


# Run all tests in root module and in known submodules
.PHONY: tests
tests::
	clear
	go test ./... $(SUBMODULES)

# target alias: test = tests
.PHONY: test
test: tests

${DIST_DIR}:
	mkdir ${DIST_DIR}


.PHONY: version
version:
	@echo "'${APP_NAME}' version from 'VERSION' file: '${APP_VERSION}'"


.PHONY: clean
clean:
	rm -rf ${DIST_DIR}

.PHONY: deps
deps:
	go get -u -t ./...
	go mod tidy
