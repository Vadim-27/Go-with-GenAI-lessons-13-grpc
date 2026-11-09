# Двоетапна збірка: на першому етапі компілюємо статичний бінарник,
# у фінальний образ FROM scratch копіюємо ЛИШЕ його — без shell, без ОС і без
# папок web/, templates/, migrations/. Шаблони, CSS/JS та SQL-міграції вбудовані
# в бінарник через go:embed, тож якщо застосунок працює тут, він не залежить від файлів.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/libratrack ./cmd/api

FROM scratch
COPY --from=build /out/libratrack /libratrack
# Непривілейований користувач (nobody).
USER 65534:65534
EXPOSE 8080
# curl у scratch немає — бінарник сам перевіряє GET /health.
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 \
  CMD ["/libratrack", "-healthcheck"]
ENTRYPOINT ["/libratrack"]
