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
	if err := db.AutoMigrate(&Position{}, &MarketData{}, &AnalysisResult{}, &AppSettings{}); err != nil {
		log.Fatalf("gagal migrasi: %v", err)
	}
	return db
}

func main() {
	db := initDB(env("DB_PATH", "portofolio.db"))
	webDir := env("WEB_DIR", "./web/out")

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
	api.Get("/settings", getSettings(db))
	api.Put("/settings", updateSettings(db))

	startScheduler(db) // background refresh; no-ops when disabled

	// Serve the Next.js static export (single-process monolith).
	// ponytail: if WEB_DIR is absent (e.g. bare `go run` without frontend
	// build), API still works; UI is served once Next.js is built.
	if _, err := os.Stat(webDir); err == nil {
		app.Static("/", webDir, fiber.Static{Compress: true})
	}

	port := env("PORT", "8080")
	log.Printf("Portofolio IHSG mendengarkan di :%s (DB=%s)", port, env("DB_PATH", "portofolio.db"))
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("server gagal mulai: %v", err)
	}
}
