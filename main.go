package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// initDB opens SQLite via the pure-Go glebarez driver (no CGO) and runs
// auto-migration for the Position model.
func initDB(path string) *gorm.DB {
	if dir := filepath.Dir(path); dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Warn)})
	if err != nil {
		log.Fatalf("gagal membuka database: %v", err)
	}
	if err := db.AutoMigrate(&Position{}, &MarketData{}, &AnalysisResult{}, &AppSettings{}, &Opportunity{}); err != nil {
		log.Fatalf("gagal migrasi: %v", err)
	}
	return db
}

// setupApp wires the Fiber app: middleware + REST routes + static frontend.
// Shared by main() (real server) and the E2E test (in-process app.Test). It
// does NOT start the scheduler or the listener — callers own those.
func setupApp(db *gorm.DB) *fiber.App {
	app := fiber.New(fiber.Config{AppName: "Portofolio IHSG"})
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New()) // local single-user app; frontend same-origin in prod

	api := app.Group("/api/v1")
	api.Get("/portfolio", listPortfolio(db))
	api.Post("/portfolio", createPosition(db))
	api.Put("/portfolio/:id", updatePosition(db))
	api.Delete("/portfolio/:id", deletePosition(db))
	api.Post("/market-data/refresh", refreshMarketData(db))
	api.Get("/stocks/:ticker", getStockDetail(db))
	api.Post("/stocks/:ticker/ai-analyze", analyzeStockAI(db))
	api.Get("/opportunities", listOpportunities(db))
	api.Get("/opportunities/lookup", lookupOpportunity(db))
	api.Post("/opportunities/refresh", refreshOpportunities(db))
	api.Get("/settings", getSettings(db))
	api.Put("/settings", updateSettings(db))

	// Serve the Next.js static export (single-process monolith).
	// ponytail: if WEB_DIR is absent (e.g. bare `go run` without frontend
	// build), API still works; UI is served once Next.js is built.
	webDir := env("WEB_DIR", "./web/out")
	if _, err := os.Stat(webDir); err == nil {
		app.Static("/", webDir, fiber.Static{Compress: true})
	}
	return app
}

func main() {
	db := initDB(env("DB_PATH", "portofolio.db"))
	seedOpportunities(db) // seed/maintain the LQ45/Kompas100 liquid universe (T5)

	app := setupApp(db)
	startScheduler(db) // background refresh; no-ops when disabled

	port := env("PORT", "8080")
	log.Printf("Portofolio IHSG mendengarkan di :%s (DB=%s)", port, env("DB_PATH", "portofolio.db"))
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("server gagal mulai: %v", err)
	}
}
