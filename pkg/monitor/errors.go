package k8deploy

import(
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
)

func execOrFail(ctx context.Context, db *pgxpool.Pool, msg string, query string, args ...any) {
	if _, err := db.Exec(ctx, query, args...); err != nil {
		log.Printf("❌ CRITICAL: %s: %v", msg, err)
		panic(msg) // or log.Fatal(msg)
	}
}
