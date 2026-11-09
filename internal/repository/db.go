package repository

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connection pool settings. MaxOpenConns must stay below the server-side
// max_connections (100 by default in PostgreSQL) shared by all instances.
const (
	maxOpenConns    = 25
	maxIdleConns    = 10
	connMaxLifetime = 30 * time.Minute
	connMaxIdleTime = 5 * time.Minute
)

// SQLLogger повертає GORM-логер рівня Info, що пише в w.
//
// ParameterizedQueries: true підставляє в лог плейсхолдери ($1, $2...) замість
// значень. Без цього в логи потрапляли б email та bcrypt-хеші паролів із
// INSERT/SELECT по users.
func SQLLogger(w io.Writer) logger.Interface {
	return logger.New(log.New(w, "", log.LstdFlags), logger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  logger.Info,
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      true,
		Colorful:                  false,
	})
}

// Open connects to PostgreSQL, configures the connection pool and turns on
// the GORM logger at Info level, so every SQL statement is printed to stdout.
func Open(dsn string) (*gorm.DB, error) {
	sqlLogger := SQLLogger(os.Stdout)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: sqlLogger})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}
