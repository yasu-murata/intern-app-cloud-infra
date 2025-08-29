package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Item 構造体
type Item struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

var db *pgxpool.Pool

func main() {
	var err error
	// ローカル開発かどうかを環境変数で判定
	if os.Getenv("ENV") == "local" {
		db, err = connectLocal() // ローカル接続用の関数を呼び出す
	} else {
		db, err = connectWithConnector() // クラウド接続用の関数を呼び出す
	}

	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer db.Close()

	http.HandleFunc("/api/items", getItems)
	http.HandleFunc("/api/items/", getItem)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

// connectLocal はローカルのPostgreSQLに接続する
func connectLocal() (*pgxpool.Pool, error) {
	// docker-compose.yml で設定した値
	dsn := "postgres://appuser:YourSecurePassword123@localhost:5432/appdb"
	dbPool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	log.Println("Successfully connected to local PostgreSQL!")
	return dbPool, nil
}

// connectWithConnector は Cloud SQL Go Connector を使ってDB接続プールを初期化する
func connectWithConnector() (*pgxpool.Pool, error) {
	mustGetenv := func(k string) string {
		v := os.Getenv(k)
		if v == "" {
			log.Fatalf("Fatal Error: %s environment variable not set.", k)
		}
		return v
	}

	var (
		dbUser                 = mustGetenv("DB_USER")
		dbPwd                  = mustGetenv("DB_PASS")
		dbName                 = mustGetenv("DB_NAME")
		instanceConnectionName = mustGetenv("INSTANCE_CONNECTION_NAME")
	)

	dsn := fmt.Sprintf("user=%s password=%s database=%s sslmode=disable", dbUser, dbPwd, dbName)
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}

	// Cloud SQL Go Connector Dialer を作成
	d, err := cloudsqlconn.NewDialer(context.Background(),
		// cloudsqlconn.WithIAMAuthN(),
	)
	if err != nil {
		return nil, err
	}

	// pgxドライバがコネクタを使うように設定
	config.ConnConfig.DialFunc = func(ctx context.Context, _, _ string) (net.Conn, error) {
		// return d.Dial(ctx, instanceConnectionName)
		return d.Dial(ctx, instanceConnectionName, cloudsqlconn.WithPrivateIP(),)
	}

	dbPool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.NewWithConfig: %w", err)
	}
	log.Println("Successfully connected to Cloud SQL!")
	return dbPool, nil
}

// getItems は全アイテムを返す
func getItems(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(context.Background(), "SELECT id, name, description FROM items ORDER BY id")
	if err != nil {
		http.Error(w, "Database query failed", http.StatusInternalServerError)
		log.Printf("db.Query failed: %v", err)
		return
	}
	defer rows.Close()

	items := []Item{}
	for rows.Next() {
		var i Item
		if err := rows.Scan(&i.ID, &i.Name, &i.Description); err != nil {
			http.Error(w, "Database scan failed", http.StatusInternalServerError)
			log.Printf("rows.Scan failed: %v", err)
			return
		}
		items = append(items, i)
	}

	log.Printf("Successfully fetched %d items", len(items))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

// getItem は単一アイテムを返す
func getItem(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/items/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid item ID", http.StatusBadRequest)
		return
	}

	var i Item
	err = db.QueryRow(context.Background(), "SELECT id, name, description FROM items WHERE id = $1", id).Scan(&i.ID, &i.Name, &i.Description)
	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Database query failed", http.StatusInternalServerError)
			log.Printf("db.QueryRow failed: %v", err)
		}
		return
	}

	log.Printf("Successfully fetched item with id: %d", id)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(i)
}