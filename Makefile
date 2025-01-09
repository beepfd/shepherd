GO := go
GO_BUILD = CGO_ENABLED=1 $(GO) build
GO_GENERATE = $(GO) generate
GO_TAGS ?=
TARGET_GOARCH ?= amd64,arm64
GOARCH ?= amd64
GOOS ?= linux
VERSION=$(shell git describe --tags --always)
# For compiling libpcap and CGO
CC ?= gcc

# 变量定义
SSH_PASS := sshpass -p root
SSH_OPTS := -o 'StrictHostKeyChecking no' -P 10022
REMOTE_HOST := root@127.0.0.1
MAX_RETRIES := 3

QEMU_LOG := qemu.log

# 定义超时时间（5分钟 = 300秒）
TIMEOUT := 300
LOG_FILE := /root/log/info.log

# .PHONY 声明
.PHONY: deploy check-connection copy-files clean retry all

# 默认目标
all: deploy

# 主要部署流程
deploy: check-connection copy-files
	@echo "Deployment complete"

# 连接检查
check-connection:
	@echo "Checking remote host connection..."
	while ! nc -z 127.0.0.1 10022 ; do echo "waiting for ssh"; sleep 1; done

# 文件复制
copy-files:
	@echo "Copying files to remote host..."
	@for i in $$(seq 1 $(MAX_RETRIES)); do \
		echo "Attempt $$i of $(MAX_RETRIES)"; \
		if $(SSH_PASS) scp $(SSH_OPTS) ./cmd/shepherd $(REMOTE_HOST):/root/shepherd && \
		   $(SSH_PASS) scp $(SSH_OPTS) ./cmd/config.yaml $(REMOTE_HOST):/root/config.yaml; then \
			echo "Files copied successfully"; \
			exit 0; \
		fi; \
		echo "Copy failed, retrying..."; \
		sleep 2; \
	done; \
	echo "All retry attempts failed"; \
	exit 1

build: elf
	cd ./cmd;CGO_ENABLED=1 GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_LDFLAGS='-g -lcapstone -static'   go build -tags=netgo,osusergo -gcflags "all=-N -l" -v  -o shepherd

dlv:  build
	dlv --headless --listen=:2345 --api-version=2 exec ./cmd/shepherd -- --config-path=./cmd/config.yaml

run:  build
	./cmd/shepherd --config-path=./cmd/config.yaml

elf:
	TARGET_GOARCH=$(TARGET_GOARCH) $(GO_GENERATE)
    	CC=$(CC) GOARCH=$(TARGET_GOARCH) $(GO_BUILD) $(if $(GO_TAGS),-tags $(GO_TAGS)) \
    		-ldflags "-w -s "

image:
	docker buildx create --use
	docker buildx build --platform linux/amd64 -t ghostbaby/shepherd:v0.0.1-amd64 --push .
	docker buildx build --platform linux/arm64 -t ghostbaby/shepherd:v0.0.1-arm64 --push .

.ONESHELL:
prepare_e2e_filesystem:
	cd ./tests/e2e/vm/filesystem
	# build filesystem image and store as tar archive
	DOCKER_BUILDKIT=1 docker build --output "type=tar,dest=filesystem.tar" .
	# convert tar to qcow2 image
	sudo virt-make-fs --format=qcow2 --size=+100M filesystem.tar filesystem-large.qcow2
	# reduce size of image
	qemu-img convert filesystem-large.qcow2 -O qcow2 filesystem.qcow2
	# reduce size by packing
	zip filesystem.zip filesystem.qcow2
	# remove unnecessary files
	rm -f filesystem-large.qcow2 filesystem.qcow2 filesystem.tar

.ONESHELL:
start_qemu:
	cd ./tests/e2e/vm/filesystem
	rm -f filesystem.qcow2 filesystem-diff.qcow2
	unzip ./filesystem.zip
	sudo qemu-img create -f qcow2 -b filesystem.qcow2 -F qcow2 filesystem-diff.qcow2
	PWD=$(pwd)
	sudo qemu-system-x86_64 \
	-cpu host \
	-m 4G \
	-smp 4 \
	-kernel ${PWD}/tests/e2e/vm/kernels/${KERNEL}/bzImage \
	-append "console=ttyS0 root=/dev/sda rw" \
	-drive file="${PWD}/tests/e2e/vm/filesystem/filesystem-diff.qcow2,format=qcow2" \
	-net nic -net user,hostfwd=tcp::10022-:22,hostfwd=tcp::16676-:6676,hostfwd=tcp::10443-:443 \
	-enable-kvm \
	-pidfile qemu.pid \
	-nographic  > $(QEMU_LOG) 2>&1 &

.ONESHELL:
prepare_e2e: start_qemu deploy
	sshpass -p root ssh -p 10022 root@127.0.0.1 'chmod 0655 /root/shepherd && systemctl start shepherd.service'
	while ! sshpass -p root ssh -p 10022 root@127.0.0.1 'systemctl is-active shepherd.service' ; do echo "waiting for shepherd service"; sleep 1; done

.ONESHELL:
e2e: prepare_e2e check-log
	ifconfig
	RC=$$?
	sshpass -p root ssh -p 10022 root@127.0.0.1 'systemctl status shepherd.service'
	sudo cat ./tests/e2e/vm/filesystem/qemu.pid | sudo xargs kill
	exit $$RC

check-log:
	@echo "Checking log file..."
	@start_time=$$(date +%s); \
	while true; do \
		current_time=$$(date +%s); \
		elapsed_time=$$((current_time - start_time)); \
		if [ $$elapsed_time -gt $(TIMEOUT) ]; then \
			echo "Timeout after $(TIMEOUT) seconds"; \
			exit 1; \
		fi; \
		if sshpass -p root ssh -p 10022 root@127.0.0.1 "\
			if [ -f $(LOG_FILE) ] && [ -s $(LOG_FILE) ]; then \
				cat $(LOG_FILE); \
				exit 0; \
			else \
				exit 1; \
			fi"; then \
			echo "Log file found and not empty"; \
			break; \
		else \
			echo "Waiting for log file... ($$elapsed_time seconds elapsed)"; \
			sleep 5; \
		fi; \
	done
