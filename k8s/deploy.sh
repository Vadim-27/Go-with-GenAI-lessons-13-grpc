#!/usr/bin/env bash
# Деплой LibraTrack у k3s. Запускати у VM: bash /app/k8s/deploy.sh [тег]
#   1) збирає образ libratrack:<тег> (за замовчуванням hw10);
#   2) імпортує його в containerd k3s (registry не потрібен);
#   3) створює Secret libratrack-secrets, якщо його ще немає (паролі — випадкові);
#   4) застосовує маніфести і чекає на rollout.
# Новий тег (наприклад hw10.1) після першого деплою = rolling update.
set -euo pipefail
cd "$(dirname "$0")/.."

TAG="${1:-hw10}"
NS=libratrack
IMAGE="libratrack:${TAG}"

# У VM з 1 CPU збірка Go дуже повільна: якщо образ уже є (зібраний на хості й
# завантажений через docker load), використовуємо його.
if docker image inspect "${IMAGE}" >/dev/null 2>&1; then
  echo "==> Образ ${IMAGE} уже є, збірку пропущено"
else
  echo "==> Збірка ${IMAGE}"
  docker build -t "${IMAGE}" .
fi

echo "==> Імпорт образу в k3s"
docker save "${IMAGE}" | sudo k3s ctr images import -

kubectl apply -f k8s/00-namespace.yaml

if ! kubectl -n "$NS" get secret libratrack-secrets >/dev/null 2>&1; then
  echo "==> Створення Secret libratrack-secrets"
  # Пароль адміністратора — з .env (BOOTSTRAP_ADMIN_PASSWORD), інакше випадковий і виводиться один раз.
  ADMIN_PASSWORD="$(grep -s '^BOOTSTRAP_ADMIN_PASSWORD=' .env | cut -d= -f2- || true)"
  if [ -z "${ADMIN_PASSWORD}" ]; then
    ADMIN_PASSWORD="$(openssl rand -hex 12)"
    echo "    пароль admin@libratrack.local: ${ADMIN_PASSWORD}"
  fi
  kubectl -n "$NS" create secret generic libratrack-secrets \
    --from-literal=POSTGRES_PASSWORD="$(openssl rand -hex 16)" \
    --from-literal=JWT_SECRET="$(openssl rand -hex 32)" \
    --from-literal=BOOTSTRAP_ADMIN_PASSWORD="${ADMIN_PASSWORD}"
fi

echo "==> Застосування маніфестів"
kubectl apply -f k8s/10-configmap.yaml -f k8s/20-postgres.yaml -f k8s/40-ingress.yaml
# Тег підставляємо в маніфест, щоб apply одразу йшов на потрібну версію (один rollout).
sed "s|image: libratrack:hw10$|image: ${IMAGE}|" k8s/30-app.yaml | kubectl apply -f -

kubectl -n "$NS" rollout status statefulset/postgres --timeout=180s
kubectl -n "$NS" rollout status deploy/libratrack --timeout=180s

kubectl -n "$NS" get pods,svc,ingress -o wide
echo
echo "Перевірка: curl -s localhost:30080/health   |   з Windows: http://localhost:30080/catalog або http://localhost:8088/catalog"
