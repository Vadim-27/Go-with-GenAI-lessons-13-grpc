#!/usr/bin/env bash
# Provision для Vagrant VM (Ubuntu 22.04). Ідемпотентний: кожен крок перевіряє, чи вже виконаний.
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

GO_VERSION="1.26.0"
K3S_VERSION="v1.31.4+k3s1"
VM_USER="vagrant"

log() { echo "==> $*"; }

log "Базові пакети"
apt-get update -qq
apt-get install -y -qq ca-certificates curl gnupg unzip zip jq python3-pip >/dev/null

# --- Docker Engine + compose plugin (офіційний apt-репозиторій) ---
if ! command -v docker >/dev/null 2>&1; then
  log "Встановлення Docker"
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor --yes -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
    > /etc/apt/sources.list.d/docker.list
  apt-get update -qq
  apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin >/dev/null
else
  log "Docker вже встановлено: $(docker --version)"
fi
systemctl enable --now docker >/dev/null
usermod -aG docker "$VM_USER"

# --- Go (для Demo 2: збірка Lambda) ---
if ! /usr/local/go/bin/go version 2>/dev/null | grep -q "go${GO_VERSION} "; then
  log "Встановлення Go ${GO_VERSION}"
  curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tgz
  rm -rf /usr/local/go
  tar -C /usr/local -xzf /tmp/go.tgz
  rm -f /tmp/go.tgz
else
  log "Go вже встановлено: $(/usr/local/go/bin/go version)"
fi
echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' > /etc/profile.d/go.sh

# --- AWS CLI v2 + awslocal (для Floci) ---
if ! command -v aws >/dev/null 2>&1; then
  log "Встановлення AWS CLI v2"
  curl -fsSL "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o /tmp/awscliv2.zip
  unzip -q -o /tmp/awscliv2.zip -d /tmp
  /tmp/aws/install --update
  rm -rf /tmp/aws /tmp/awscliv2.zip
else
  log "AWS CLI вже встановлено: $(aws --version)"
fi
if ! command -v awslocal >/dev/null 2>&1; then
  log "Встановлення awscli-local"
  pip3 install -q awscli-local
fi

# Фейкові ключі лише для локального емулятора (слайд 38).
VM_HOME="/home/${VM_USER}"
if [ ! -f "${VM_HOME}/.aws/credentials" ]; then
  log "Налаштування ~/.aws для Floci (test/test)"
  mkdir -p "${VM_HOME}/.aws"
  printf '[default]\naws_access_key_id = test\naws_secret_access_key = test\n' > "${VM_HOME}/.aws/credentials"
  printf '[default]\nregion = eu-central-1\noutput = json\n' > "${VM_HOME}/.aws/config"
  chown -R "${VM_USER}:${VM_USER}" "${VM_HOME}/.aws"
fi

# --- k3s ---
if ! command -v k3s >/dev/null 2>&1; then
  log "Встановлення k3s ${K3S_VERSION}"
  curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION="${K3S_VERSION}" sh -s - --write-kubeconfig-mode 644
else
  log "k3s вже встановлено: $(k3s --version | head -n1)"
fi

log "Очікування готовності вузла k3s"
for _ in $(seq 1 60); do
  if k3s kubectl get nodes 2>/dev/null | grep -q " Ready"; then break; fi
  sleep 5
done

# kubeconfig для користувача vagrant (слайд 40)
mkdir -p "${VM_HOME}/.kube"
cp /etc/rancher/k3s/k3s.yaml "${VM_HOME}/.kube/config"
chown -R "${VM_USER}:${VM_USER}" "${VM_HOME}/.kube"
chmod 600 "${VM_HOME}/.kube/config"

if ! k3s kubectl get namespace demo >/dev/null 2>&1; then
  k3s kubectl create namespace demo
fi

# k3s переписує iptables і губить правила Docker для мереж Compose (лишається тільки docker0),
# а FORWARD має політику DROP: контейнери однієї мережі Compose не бачать один одного
# ("dial tcp 172.18.0.2:5432: connection timed out"). Перезапуск Docker відновлює правила.
log "Перезапуск Docker після k3s (відновлення правил iptables)"
systemctl restart docker

log "Готово"
docker --version
docker compose version
/usr/local/go/bin/go version
aws --version
k3s kubectl get nodes -o wide
