#!/usr/bin/env bash
# Вимірює холодні та теплі виклики на Floci (крок 4 практики, слайд 44).
# Холодний старт імітуємо видаленням «теплих» контейнерів функції перед викликом.
# Використання: scripts/bench-cold-start.sh [N=10] [функція=libratrack-webhook]
set -uo pipefail
cd "$(dirname "$0")/.."
source scripts/env.sh

N=${1:-10}
FN=${2:-$WEBHOOK_FN}
EVENT=events/valid.json
[[ "$FN" == "$THUMB_FN" ]] && EVENT=events/s3_put.json
command -v docker >/dev/null || { echo "✗ Потрібен docker для скидання теплих контейнерів"; exit 1; }

now_ms() {
  local t; t=$(date +%s%3N 2>/dev/null)
  if [[ "$t" =~ ^[0-9]+$ ]]; then echo "$t"; else python3 -c 'import time;print(int(time.time()*1000))'; fi
}
invoke_ms() {
  local s e; s=$(now_ms)
  awsf lambda invoke --function-name "$FN" --payload "fileb://$EVENT" \
    --cli-binary-format raw-in-base64-out /dev/null >/dev/null 2>&1
  e=$(now_ms); echo $((e - s))
}
reset_warm() { docker ps -aq --filter "name=${FN}" | xargs -r docker rm -f >/dev/null 2>&1; }
stats() { awk '{s+=$1; if(min==""||$1<min)min=$1; if($1>max)max=$1} END{printf "avg=%d мс  min=%d мс  max=%d мс", s/NR, min, max}'; }

echo "Функція: $FN, повторів: $N (архітектура ${LAMBDA_ARCH})"
invoke_ms >/dev/null   # прогрів: образ середовища точно завантажений
cold=(); warm=()
for i in $(seq 1 "$N"); do
  reset_warm
  c=$(invoke_ms); w=$(invoke_ms)
  cold+=("$c"); warm+=("$w")
  printf "  #%-2d  cold=%5s мс   warm=%5s мс\n" "$i" "$c" "$w"
done
echo
echo "COLD: $(printf '%s\n' "${cold[@]}" | stats)"
echo "WARM: $(printf '%s\n' "${warm[@]}" | stats)"
cat <<'NOTE'

⚠️  Як трактувати результат (слайд 44, «Compare like with like»):
  • На Floci «холодний старт» включає запуск Docker-контейнера, а не Firecracker
    micro-VM. Абсолютні числа НЕ співпадають з реальним AWS.
  • Корисний висновок — ВІДНОСНИЙ: різниця cold vs warm та вплив розміру бінарника,
    пам'яті та залежностей. Порівнюйте з опублікованим бенчмарком обережно і
    запишіть це застереження у своєму порівнянні.
NOTE
