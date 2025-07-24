BIN_DIR = bin
PROTO_DIR = internal/proto
THIRD_PARTY_DIR = internal/third_party
SERVER_DIR = server

ifeq ($(OS), Windows_NT)
	SHELL := powershell.exe
	.SHELLFLAGS := -NoProfile -Command
	SHELL_VERSION = $(shell (Get-Host | Select-Object Version | Format-Table -HideTableHeaders | Out-String).Trim())
	OS = $(shell "{0} {1}" -f "windows", (Get-ComputerInfo -Property OsVersion, OsArchitecture | Format-Table -HideTableHeaders | Out-String).Trim())
	PACKAGE = $(shell (Get-Content go.mod -head 1).Split(" ")[1])
	CHECK_DIR_CMD = if (!(Test-Path $@)) { $$e = [char]27; Write-Error "$$e[31mDirectory $@ doesn't exist$${e}[0m" }
	HELP_CMD = Select-String "^[a-zA-Z_-]+:.*?\#\# .*$$" "./Makefile" | Foreach-Object { $$_data = $$_.matches -split ":.*?\#\# "; $$obj = New-Object PSCustomObject; Add-Member -InputObject $$obj -NotePropertyName ('Command') -NotePropertyValue $$_data[0]; Add-Member -InputObject $$obj -NotePropertyName ('Description') -NotePropertyValue $$_data[1]; $$obj } | Format-Table -HideTableHeaders @{Expression={ $$e = [char]27; "$$e[36m$$($$_.Command)$${e}[0m" }}, Description
	RM_F_CMD = Remove-Item -erroraction silentlycontinue -Force
	RM_RF_CMD = ${RM_F_CMD} -Recurse
	SERVER_BIN = ${SERVER_DIR}.exe
	DOCKER_COMPOSE_CMD = $(shell if (Get-Command docker-compose -ErrorAction SilentlyContinue) { "docker-compose" } elseif (docker compose version 2>$$null) { "docker compose" } else { "docker-compose" })
else
	SHELL := bash
	SHELL_VERSION = $(shell echo $$BASH_VERSION)
	UNAME := $(shell uname -s)
	VERSION_AND_ARCH = $(shell uname -rm)
	ifeq ($(UNAME),Darwin)
		OS = macos ${VERSION_AND_ARCH}
	else ifeq ($(UNAME),Linux)
		OS = linux ${VERSION_AND_ARCH}
	else
		$(error OS not supported by this Makefile)
	endif
	PACKAGE = $(shell head -1 go.mod | awk '{print $$2}')
	CHECK_DIR_CMD = test -d $@ || (echo "\033[31mDirectory $@ doesn't exist\033[0m" && false)
	HELP_CMD = grep -E '^[a-zA-Z_-]+:.*?\#\# .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?\#\# "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
	RM_F_CMD = rm -f
	RM_RF_CMD = ${RM_F_CMD} -r
	SERVER_BIN = ${SERVER_DIR}
	DOCKER_COMPOSE_CMD = $(shell if command -v docker-compose >/dev/null 2>&1; then echo "docker-compose"; elif docker compose version >/dev/null 2>&1; then echo "docker compose"; else echo "docker-compose"; fi)
endif

.DEFAULT_GOAL := help
.PHONY: rinha help
project := rinha

all: $(project)

install-tools:
	@echo "Installing protobuf tools..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest

rinha: $@ 

$(project):
	@echo "Setting up third party dependencies..."
	@mkdir -p ${THIRD_PARTY_DIR}/google/api
	@if [ ! -f ${THIRD_PARTY_DIR}/google/api/annotations.proto ]; then \
		curl -L https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/annotations.proto -o ${THIRD_PARTY_DIR}/google/api/annotations.proto; \
	fi
	@if [ ! -f ${THIRD_PARTY_DIR}/google/api/http.proto ]; then \
		curl -L https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/http.proto -o ${THIRD_PARTY_DIR}/google/api/http.proto; \
	fi
	@echo "Generating protobuf files with gRPC-Gateway..."
	protoc --proto_path=${PROTO_DIR} --proto_path=${THIRD_PARTY_DIR} --proto_path=. \
		--go_out=${PROTO_DIR} --go_opt=paths=source_relative \
		--go-grpc_out=${PROTO_DIR} --go-grpc_opt=paths=source_relative \
		--grpc-gateway_out=${PROTO_DIR} --grpc-gateway_opt=paths=source_relative \
		--grpc-gateway_opt=generate_unbound_methods=true \
		${PROTO_DIR}/*.proto
	@echo "Building server..."
	go build -o ${BIN_DIR}/${SERVER_BIN} ./cmd/api

test: all
	go test ./...

run: all
	@echo "Starting gRPC server on :8000 and HTTP gateway on :8081"
	./${BIN_DIR}/${SERVER_BIN}

clean: clean_rinha
	${RM_F_CMD} ssl/*.crt
	${RM_F_CMD} ssl/*.csr
	${RM_F_CMD} ssl/*.key
	${RM_F_CMD} ssl/*.pem
	${RM_RF_CMD} ${BIN_DIR}

clean_rinha:
	${RM_F_CMD} ./${PROTO_DIR}/*.pb.go
	${RM_F_CMD} ./${PROTO_DIR}/*.gw.go

rebuild: clean all

bump: all
	go get -u ./...

start-containers:
	${DOCKER_COMPOSE_CMD} down -v && ${DOCKER_COMPOSE_CMD} up -d

open-backend-logs:
	${DOCKER_COMPOSE_CMD} logs -f backend-1 backend-2

about:
	@echo "OS: ${OS}"
	@echo "Shell: ${SHELL} ${SHELL_VERSION}"
	@echo "Docker Compose: ${DOCKER_COMPOSE_CMD}"
	@echo "Protoc version: $(shell protoc --version)"
	@echo "Go version: $(shell go version)"
	@echo "Go package: ${PACKAGE}"
	@echo "Openssl version: $(shell openssl version)"

commands:
	@echo -e "\033[36m=== Available Make Commands ===\033[0m"
	@echo ""
	@echo -e "\033[33mBuild & Development:\033[0m"
	@echo -e "  \033[32mall\033[0m                    - Build the entire project (default target)"
	@echo -e "  \033[32minstall-tools\033[0m          - Install required protobuf and gRPC tools"
	@echo -e "  \033[32mrinha\033[0m                  - Setup dependencies and generate protobuf files"
	@echo -e "  \033[32mtest\033[0m                   - Run all Go tests"
	@echo -e "  \033[32mrun\033[0m                    - Build and run server (gRPC :8000, HTTP :8081)"
	@echo -e "  \033[32mrebuild\033[0m                - Clean and rebuild everything from scratch"
	@echo -e "  \033[32mbump\033[0m                   - Update all Go dependencies to latest versions"
	@echo ""
	@echo -e "\033[33mCleaning:\033[0m"
	@echo -e "  \033[32mclean\033[0m                  - Clean all generated files and binaries"
	@echo -e "  \033[32mclean_rinha\033[0m            - Clean only generated protobuf files"
	@echo ""
	@echo -e "\033[33mDocker Operations:\033[0m"
	@echo -e "  \033[32mstart-containers\033[0m       - Stop and start Docker containers (reset volumes)"
	@echo -e "  \033[32mopen-backend-logs\033[0m      - Show logs from backend-1 and backend-2 containers"
	@echo ""
	@echo -e "\033[33mUtilities:\033[0m"
	@echo -e "  \033[32mabout\033[0m                  - Show system information and tool versions"
	@echo -e "  \033[32mhelp\033[0m                   - Show quick help with command descriptions"
	@echo -e "  \033[32mcommands\033[0m               - Show this detailed explanation"
	@echo ""
	@echo -e "\033[36mUsage Examples:\033[0m"
	@echo "  make                     # Show help"
	@echo "  make all                 # Build everything"
	@echo "  make run                 # Build and start server"
	@echo "  make start-containers    # Start Docker environment"
	@echo "  make test                # Run tests"

help:
	@${HELP_CMD}
