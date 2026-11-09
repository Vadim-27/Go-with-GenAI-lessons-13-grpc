package main

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"libratrack/internal/app"
	"libratrack/internal/auth"
	"libratrack/internal/model"
	"libratrack/internal/repository"
	"libratrack/migrations"
	"libratrack/web"
)

func main() {
	// Docker HEALTHCHECK: в образі FROM scratch немає curl, тому бінарник
	// сам перевіряє /health і повертає код виходу 0 або 1.
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		os.Exit(healthcheck())
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	// Без надійного JWT_SECRET (>= 32 символи) сервер не стартує.
	if err := auth.ValidateSecret(); err != nil {
		log.Fatalf("config: %v", err)
	}
	addr := ":" + getenv("PORT", "8080")

	// Міграції вбудовані в бінарник (go:embed). MIGRATIONS_PATH дозволяє
	// за потреби взяти їх із диска (наприклад, під час розробки нової міграції).
	var migrationsFS fs.FS = migrations.FS
	if dir := os.Getenv("MIGRATIONS_PATH"); dir != "" {
		migrationsFS = os.DirFS(dir)
	}
	// The schema is created only by SQL migrations, applied on every start.
	if err := repository.Migrate(dsn, migrationsFS); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	db, err := repository.Open(dsn)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	if err := bootstrapAdmin(repository.NewUserRepository(db)); err != nil {
		log.Fatalf("bootstrap admin: %v", err)
	}

	// Шаблони та статика вбудовані через go:embed (пакет web): fs.Sub
	// прибирає префікс static/, щоб файли роздавались за /static/css/app.css.
	staticFS, err := web.Static()
	if err != nil {
		log.Fatalf("static files: %v", err)
	}
	router, err := app.NewRouter(db, app.Web{
		Templates: web.TemplatesFS,
		Static:    staticFS,
		// Secure за замовчуванням; COOKIE_SECURE=false — лише для HTTP не на localhost.
		SecureCookie: os.Getenv("COOKIE_SECURE") != "false",
	})
	if err != nil {
		log.Fatalf("router: %v", err)
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Graceful shutdown on Ctrl+C / SIGTERM: finish in-flight requests first.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("listening on %s (web UI: http://localhost%s/catalog)", addr, addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

// bootstrapAdmin створює першого адміністратора з BOOTSTRAP_ADMIN_EMAIL та
// BOOTSTRAP_ADMIN_PASSWORD, якщо такого email ще немає. Без цих змінних нічого
// не робить. Пароль у коді та міграціях не зберігається.
func bootstrapAdmin(users repository.UserRepository) error {
	email, password := os.Getenv("BOOTSTRAP_ADMIN_EMAIL"), os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")
	if email == "" || password == "" {
		return nil
	}
	in := model.RegisterRequest{Email: &email, Password: &password}
	if err := in.Validate(); err != nil {
		return err
	}

	ctx := context.Background()
	user := in.ToUser("")
	if _, err := users.FindByEmail(ctx, user.Email); err == nil {
		return nil // уже існує: роль і пароль не змінюємо
	} else if !errors.Is(err, repository.ErrNotFound) {
		return err
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	user.PasswordHash = hash
	user.Role = model.RoleAdmin
	if err := users.Create(ctx, &user); err != nil {
		return err
	}
	log.Printf("bootstrap: created admin %s", user.Email)
	return nil
}

func healthcheck() int {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + getenv("PORT", "8080") + "/health")
	if err != nil {
		log.Printf("healthcheck: %v", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("healthcheck: status %d", resp.StatusCode)
		return 1
	}
	return 0
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
