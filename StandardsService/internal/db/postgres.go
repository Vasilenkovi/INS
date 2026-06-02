// Package db инкапсулирует подключение к PostgreSQL и применение миграций.
// main.go получает готовый *sql.DB и больше не знает про DSN и пулы.
package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

// Config — настройки подключения и пула соединений.
type Config struct {
	DSN string

	// Пул соединений
	MaxOpenConns    int           // максимум открытых соединений (default: 25)
	MaxIdleConns    int           // максимум простаивающих соединений (default: 5)
	ConnMaxLifetime time.Duration // максимальное время жизни соединения (default: 5m)
	ConnMaxIdleTime time.Duration // максимальное время простоя соединения (default: 1m)

	// Путь к директории с миграциями.
	// Например: "file://migrations" или "file:///app/migrations"
	MigrationsPath string
}

// DefaultConfig возвращает конфиг с разумными дефолтами.
func DefaultConfig(dsn, migrationsPath string) Config {
	return Config{
		DSN:             dsn,
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: time.Minute,
		MigrationsPath:  migrationsPath,
	}
}

// Open открывает соединение с PostgreSQL, настраивает пул и проверяет доступность.
func Open(cfg Config, logger *slog.Logger) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("db open: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	if err := ping(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}

	logger.Info("database connected",
		"max_open_conns", cfg.MaxOpenConns,
		"max_idle_conns", cfg.MaxIdleConns,
	)
	return db, nil
}

// Migrate применяет все pending-миграции из MigrationsPath.
// Вызывается один раз при старте сервиса после Open.
// Идемпотентна: если миграции уже применены — ничего не делает.
func Migrate(database *sql.DB, cfg Config, logger *slog.Logger) error {
	if cfg.MigrationsPath == "" {
		logger.Warn("migrations path not set, skipping")
		return nil
	}

	driver, err := postgres.WithInstance(database, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("migrate: create driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(cfg.MigrationsPath, "postgres", driver)
	if err != nil {
		return fmt.Errorf("migrate: init: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate: up: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("migrate: version: %w", err)
	}

	logger.Info("migrations applied", "version", version, "dirty", dirty)
	return nil
}

// MigrateDown откатывает все миграции. Используется в тестах.
func MigrateDown(database *sql.DB, cfg Config) error {
	driver, err := postgres.WithInstance(database, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("migrate down: create driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(cfg.MigrationsPath, "postgres", driver)
	if err != nil {
		return fmt.Errorf("migrate down: init: %w", err)
	}

	if err := m.Down(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

// ping пробует подключиться несколько раз — PostgreSQL может быть не готов сразу.
func ping(db *sql.DB) error {
	const attempts = 5
	const delay = time.Second

	var lastErr error
	for i := 0; i < attempts; i++ {
		if lastErr = db.Ping(); lastErr == nil {
			return nil
		}
		time.Sleep(delay)
	}
	return fmt.Errorf("db not reachable after %d attempts: %w", attempts, lastErr)
}
