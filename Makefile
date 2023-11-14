SHELL    = bash
GO       = go
MODULE   = $(shell env GO111MODULE=on $(GO) list -m)
PKGS     = $(or $(PKG),$(shell env GO111MODULE=on $(GO) list ./...))
TESTPKGS = $(shell env GO111MODULE=on $(GO) list -f \
			'{{ if or .TestGoFiles .XTestGoFiles }}{{ .ImportPath }}{{ end }}' \
			$(PKGS))
TARGET   = $(shell basename $(MODULE))

TIMEOUT = 300
SPACE := $(subst ,, )
V ?= 0
Q = $(if $(filter 1,$V),,@)
M = $(shell printf "\033[34;1m▶\033[0m")


define whatisit
$(info $(1) origin is ($(origin $(1))) and value is ($($(1))))
endef

# set candidate compiler
MUSL_CC := $(shell [ $$(command -v musl-gcc 2>&1 >/dev/null; echo $$?) = 0 ] && echo y || echo n)
ifeq ($(MUSL_CC),y)
	export CC=musl-gcc
endif

##################### LDFLAGS #####################
# VERSION
VERSION = $(shell git describe --tags --always --dirty="-dev")

BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD)
ifeq ($(BRANCH),HEAD) # adjust for gitlab checkout
	BRANCH = $(CI_COMMIT_REF_NAME)
endif

RELEASE_BRANCH ?= $(shell echo $(BRANCH) | sed 's/^release\///')
ifeq ($(BRANCH),$(RELEASE_BRANCH)) # not in "release/xxxx" branch
	RELEASE_BRANCH =
else
	VERSION = $(RELEASE_BRANCH)
endif

ifeq ($(BRANCH),master) # fix for master branch
	VERSION = master
endif

# BUILDNUMBER
ifeq ($(CI_PIPELINE_ID),)
    BUILDNUMER := private
else
    BUILDNUMER := $(CI_PIPELINE_ID)
endif
# DATE
DATE    ?= $(shell date +%FT%T%z)
# GIT_COMMIT
GIT_COMMIT ?= $(shell git rev-parse --short HEAD)
ifeq ($(CI_COMMIT_AUTHOR),)
    COMMIT_AUTHOR := unknown
else
    COMMIT_AUTHOR := $(subst $(SPACE),,$(CI_COMMIT_AUTHOR))
endif


LDFLAGS := -extldflags -static -X $(MODULE)/cmd.Version=$(VERSION) \
	-X $(MODULE)/cmd.BuildTime=$(DATE) \
	-X $(MODULE)/cmd.GitCommit=$(GIT_COMMIT) \
	-X $(MODULE)/cmd.BuildBy=$(COMMIT_AUTHOR) \
	-X $(MODULE)/cmd.BuildNumber=$(BUILDNUMER)

RELEASE ?= 0
ifeq ($(RELEASE),1)
	LDFLAGS += -s -w
endif
##################### LDFLAGS #####################


BIN			= $(CURDIR)/bin
PREFIX		?=	/usr/local
INSTALLDIR 	?= $(PREFIX)/bin
MANDIR 		?=	 $(PREFIX)/share/man/man1
# compress the raw binary when in release mode and upx installed
UPX     	:= $(shell [ $$(command -v upx  2>&1 >/dev/null; echo $$?) = 0 ] && [ $(RELEASE) = 1 ] && echo y || echo n)

export GO111MODULE=on
export GOPROXY=https://goproxy.cn,direct

ifeq ($(V),1)
$(call whatisit,CC)
$(call whatisit,UPX)
$(call whatisit,GIT_COMMIT)
$(call whatisit,LDFLAGS)
$(call whatisit,VERSION)
$(call whatisit,BUILDNUMER)
$(call whatisit,BRANCH)
$(call whatisit,RELEASE_BRANCH)
$(call whatisit,RELEASE)
endif

.PHONY: all
all: build

.PHONY: build
build: fmt | $(BIN) ; $(info $(M) building executable) @ ## Build program binary
	$Q GOOS=linux GOARCH=amd64 go build -o bin/$(TARGET) -ldflags "$(LDFLAGS)" -tags release,jsoniter,nomsgpack -gcflags "all=-N -l"
ifeq ($(UPX), 0)
	$Q $(info $(M) compacting executable)
	$Q upx bin/$(TARGET) 2>&1 > /dev/null
endif

compile: bin/$(TARGET)-linux-amd64-$(VERSION)-$(BUILDNUMER)

package: bin/$(TARGET)-linux-amd64-$(VERSION)-$(BUILDNUMER)
	echo $(GITCOMMIT) > commit.txt
	echo $(VERSION) > version.txt
	zip -r $(TARGET)-$(VERSION)-$(BUILDNUMER).zip bin commit.txt version.txt

bin/$(TARGET)-linux-amd64-$(VERSION)-$(BUILDNUMER):
	GOOS=linux GOARCH=amd64 go build -o bin/$(TARGET)-linux-amd64-$(VERSION)-$(BUILDNUMER) -ldflags "$(LDFLAGS)" -tags release,jsoniter,nomsgpack -gcflags "all=-N -l"
	cp bin/$(TARGET)-linux-amd64-$(VERSION)-$(BUILDNUMER) bin/$(TARGET)-linux-amd64-$(VERSION)
ifeq ($(BRANCH),master)
	cp bin/$(TARGET)-linux-amd64-$(VERSION)-$(BUILDNUMER) bin/$(TARGET)
endif
ifdef RELEASE_BRANCH
	cp bin/$(TARGET)-linux-amd64-$(VERSION)-$(BUILDNUMER) bin/$(TARGET)-$(RELEASE_BRANCH)
endif

# use `source .env` and then `make release`
release: compile
	$Q rclone --config build/rclone.conf -v copy $(BIN) 42:ellis-chen-artifects/$(TARGET)/
	$Q echo "linux bin access url is : \n"
	$Q RCLONE_LOG_LEVEL=ERROR rclone --config build/rclone.conf link 42:ellis-chen-artifects/$(TARGET)/$(TARGET)-linux-amd64-$(VERSION)

list:
	$Q rclone --config build/rclone.conf ls 42:ellis-chen-artifects/$(TARGET)/

$(INSTALLDIR):
	$Q sudo mkdir -p $@
$(MANDIR):
	$Q sudo mkdir -p $@

.PHONY: install
install: build | $(INSTALLDIR) $(MANDIR); $(info $(M) installing $(TARGET) to $(INSTALLDIR)) @ ## Install binary
	$Q sudo install -m 0755 $(BIN)/$(TARGET)  $(INSTALLDIR)
	$Q sudo FA_MAN_LOC=$(MANDIR)  $(INSTALLDIR)/$(TARGET) -m

.PHONY: manpage
manpage: build | $(INSTALLDIR) $(MANDIR);
	$Q sudo FA_MAN_LOC=$(MANDIR)  $(INSTALLDIR)/$(TARGET) -m

.PHONY: img  @ ## build image and push
img: bin/$(TARGET)-linux-amd64-$(VERSION)-$(BUILDNUMER) ; $(info $(M) building docker image) @ ## Build local docker image and push it
	cp bin/$(TARGET)-linux-amd64-$(VERSION)-$(BUILDNUMER) bin/$(TARGET)
	$Q bash -ex make.sh

.PHONY: img  @ ## build image and push
devimg: clean bin/$(TARGET)-linux-amd64-$(VERSION)-$(BUILDNUMER); $(info $(M) building docker image) @ ## Build local docker image and push it
	$Q cp bin/$(TARGET)-linux-amd64-$(VERSION)-$(BUILDNUMER) bin/$(TARGET)
	$Q bash -ex make.sh

.PHONY: version
version: ; @ ## Show current build version
	$Q echo $(VERSION)

.PHONY: clean
clean: ; $(info $(M) cleaning)	@ ## Cleanup everything
	$Q rm -rf $(BIN)
	$Q rm -rf test/tests.* test/coverage.*
	$Q rm -rf gen/

.PHONY: help
help:
	$Q grep -hE '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-17s\033[0m %s\n", $$1, $$2}'

.PHONY: chglog
chglog:
	$Q echo "Update CHANGELOG: $(BUILD_DATE)"
	$Q echo "Git tag: $(GIT_TAG)"
	$Q echo "Current Directory $(CURRENT_DIR)"
	$Q git-chglog -o CHANGELOG.md
	$Q echo "UPDATE Finished"

# Tools
$(BIN):
	$Q mkdir -p $@

$(BIN)/%: | $(BIN) ; $(info $(M) building $(PACKAGE))
	tmp=$$(mktemp -d); \
	GOPATH=$$tmp env GO111MODULE=on GOPROXY=https://goproxy.cn,direct GOBIN=$(BIN) $(GO) install $(PACKAGE) \
		|| ret=$$?; \
	chmod 700 -R $$tmp; rm -rf $$tmp ; exit $$ret

GOLINT = $(BIN)/golint
$(BIN)/golint: PACKAGE=golang.org/x/lint/golint@latest

GOLANGCILINT = $(BIN)/golangci-lint
$(BIN)/golangci-lint: PACKAGE=github.com/golangci/golangci-lint/cmd/golangci-lint@latest

GOCOV = $(BIN)/gocov
$(BIN)/gocov: PACKAGE=github.com/axw/gocov/gocov@latest

GOCOVXML = $(BIN)/gocov-xml
$(BIN)/gocov-xml: PACKAGE=github.com/AlekSi/gocov-xml@latest

GO2XUNIT = $(BIN)/go2xunit
$(BIN)/go2xunit: PACKAGE=github.com/tebeka/go2xunit@latest

GOCHECKLOCKS = $(BIN)/checklocks
$(BIN)/checklocks: PACKAGE=gvisor.dev/gvisor/tools/checklocks/cmd/checklocks@latest

# Tests

TEST_TARGETS := test-default test-bench test-short test-verbose test-race
.PHONY: $(TEST_TARGETS) test-xml check test tests
test-bench:   ARGS=-run=__absolutelynothing__ -bench=. ## Run benchmarks
test-short:   ARGS=-short        ## Run only short tests
test-verbose: ARGS=-v            ## Run tests in verbose mode with coverage reporting
test-race:    ARGS=-race -count=1        ## Run tests with race detector
$(TEST_TARGETS): NAME=$(MAKECMDGOALS:test-%=%)
$(TEST_TARGETS): test
test: fmt lint ; $(info $(M) running $(NAME:%=% )tests) @ ## Run tests
	$Q $(GO) test -mod=mod -count 1 -timeout $(TIMEOUT)s $(ARGS) $(TESTPKGS)

test-xml: fmt lint | $(GO2XUNIT) ; $(info $(M) running xUnit tests) @ ## Run tests with xUnit output
	$Q mkdir -p test
	$Q 2>&1 $(GO) test -timeout $(TIMEOUT)s -v $(TESTPKGS) | tee test/tests.output
	$(GO2XUNIT) -fail -input test/tests.output -output test/tests.xml

COVERAGE_MODE    = atomic
COVERAGE_PROFILE = $(COVERAGE_DIR)/profile.out
COVERAGE_XML     = $(COVERAGE_DIR)/coverage.xml
COVERAGE_HTML    = $(COVERAGE_DIR)/index.html
.PHONY: test-coverage test-coverage-tools
test-coverage-tools: | $(GOCOV) $(GOCOVXML)
test-coverage: COVERAGE_DIR := $(CURDIR)/test/coverage.$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
test-coverage: fmt lint test-coverage-tools ; $(info $(M) running coverage tests) @ ## Run coverage tests
	$Q mkdir -p $(COVERAGE_DIR)
	$Q $(GO) test \
		-coverpkg=$$($(GO) list -f '{{ join .Deps "\n" }}' $(TESTPKGS) | \
					grep '^$(MODULE)/' | \
					tr '\n' ',' | sed 's/,$$//') \
		-covermode=$(COVERAGE_MODE) \
		-coverprofile="$(COVERAGE_PROFILE)" $(TESTPKGS)
	$Q $(GO) tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)
	$Q $(GOCOV) convert $(COVERAGE_PROFILE) | $(GOCOVXML) > $(COVERAGE_XML)

.PHONY: lint
lint: $(GOLANGCILINT) ; $(info $(M) running golangci-lint) @ ## Run golangci-lint
	$Q $(GOLANGCILINT) run -set_exit_status --modules-download-mode mod

.PHONY: checklocks
checklocks: $(GOCHECKLOCKS) ; $(info $(M) running checklocks) @ ## Run checklocks
	$Q $(GOCHECKLOCKS) $(PKGS)

.PHONY: fmt
fmt: ; $(info $(M) running gofmt) @ ## Run gofmt on all source files
	$Q $(GO) fmt $(PKGS)

.PHONY: proto
proto: clean; $(info $(M) generating proto source file)	@ ## Generate source file from proto
	$Q echo generating source files from proto
	buf generate --template submodules/api-contract/templates/go.yaml -o third_party/audit submodules/api-contract/protobuf/auditing

dev: compile
	docker-compose -f docker-compose.yaml stop -t 3
	docker-compose -f docker-compose.yaml up --build --force-recreate -d

dev-logs:
	docker-compose -f docker-compose.yaml logs -f

dev-clean:
	docker-compose -f docker-compose.yaml down -v