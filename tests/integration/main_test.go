//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	_ = godotenv.Load("../../.env.test")
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		fmt.Println("TEST_DATABASE_URL tidak diisi: integration test dilewati")
		os.Exit(0)
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil || !strings.HasSuffix(cfg.ConnConfig.Database, "_test") {
		fmt.Println("menolak berjalan: database harus berakhiran _test")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	testPool, err = pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		fmt.Printf("konfigurasi database test gagal: %v\n", err)
		os.Exit(1)
	}

	if err := testPool.Ping(ctx); err != nil {
		fmt.Printf("gagal terhubung ke database test (%s): %v\n", dsn, err)
		os.Exit(1)
	}

	code := m.Run()
	testPool.Close()
	os.Exit(code)
}

