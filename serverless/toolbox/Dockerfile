FROM golang:1.23-bookworm
RUN apt-get update && apt-get install -y --no-install-recommends zip make curl unzip ca-certificates python3 && rm -rf /var/lib/apt/lists/*
# Свіжий клієнт Docker (Debian-івський docker.io надто старий для Docker Desktop: API 1.41 < 1.44)
RUN ARCH=$(dpkg --print-architecture) && case $ARCH in amd64) A=x86_64;; arm64) A=aarch64;; esac \
 && curl -sL "https://download.docker.com/linux/static/stable/$A/docker-27.5.1.tgz" | tar -xz -C /usr/local/bin --strip-components=1 docker/docker
RUN ARCH=$(dpkg --print-architecture) && case $ARCH in amd64) A=x86_64;; arm64) A=aarch64;; esac \
 && curl -sL "https://awscli.amazonaws.com/awscli-exe-linux-$A.zip" -o /tmp/a.zip && unzip -q /tmp/a.zip -d /tmp && /tmp/aws/install && rm -rf /tmp/a.zip /tmp/aws
RUN ARCH=$(dpkg --print-architecture) && curl -sL "https://releases.hashicorp.com/terraform/1.9.8/terraform_1.9.8_linux_${ARCH}.zip" -o /tmp/t.zip && unzip -q /tmp/t.zip -d /usr/local/bin && rm /tmp/t.zip
