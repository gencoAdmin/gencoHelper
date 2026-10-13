package main

import (
	"context"
	"database/sql"
	"genco/finance/internal/config"
	"genco/finance/migrations"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"os"
	"time"
)

func main() {
	config.LoadDotEnv("backend/.env", ".env", "../.env")
	dsn := os.Getenv("GENCO_MIGRATION_DSN")
	if dsn == "" {
		log.Fatal("GENCO_MIGRATION_DSN is required; use the migration database user")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err = migrations.Up(ctx, db); err != nil {
		log.Fatal(err)
	}
	log.Println("Migrations applied")
}
