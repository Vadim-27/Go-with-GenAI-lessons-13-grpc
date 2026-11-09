// mint-token друкує підписаний демо-JWT. Приклад:
//
//	TOKEN=$(go run ./cmd/mint-token -role librarian)
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"libratrack/internal/grpcauth"
)

func main() {
	sub := flag.String("sub", "demo-user", "subject (хто це)")
	role := flag.String("role", grpcauth.RoleLibrarian, "роль: reader | librarian")
	ttl := flag.Duration("ttl", time.Hour, "строк дії токена")
	secret := flag.String("secret", envOr("JWT_SECRET", grpcauth.DefaultDemoSecret), "HMAC-секрет (має збігатися з сервером)")
	flag.Parse()

	if *role != grpcauth.RoleReader && *role != grpcauth.RoleLibrarian {
		log.Fatalf("невідома роль %q (дозволено: reader, librarian)", *role)
	}
	tok, err := grpcauth.Sign([]byte(*secret), grpcauth.Claims{
		Subject:   *sub,
		Role:      *role,
		ExpiresAt: time.Now().Add(*ttl).Unix(),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(tok)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
