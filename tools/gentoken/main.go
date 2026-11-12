package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"findit-api/config"
	"findit-api/database"
	"findit-api/helper"
)

func main() {
	username := flag.String("user", "lifia", "username mahasiswa")
	flag.Parse()
	config.LoadEnv()
	pool, err := database.NewPool(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	var id int
	var uname, role string
	err = pool.QueryRow(context.Background(), `SELECT id, username, role FROM users WHERE username = $1`, *username).Scan(&id, &uname, &role)
	if err != nil {
		log.Fatalf("user tidak ditemukan: %v", err)
	}
	jwtMgr := helper.NewJWTManager(config.GetEnv("JWT_SECRET", ""), config.GetEnv("JWT_ISSUER", "findit-api"), 24*time.Hour)
	token, err := jwtMgr.GenerateAccess(id, uname, role)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("User: %s (ID=%d, Role=%s)\nBearer %s\n", uname, id, role, token)
}
