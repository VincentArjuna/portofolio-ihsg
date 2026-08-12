package main

import (
	"log"
	"time"

	"gorm.io/gorm"
)

// schedulerTick is how often the background loop wakes to check whether a
// scheduled refresh is due. The actual cadence is governed by settings
// (refresh_interval_hours); this only bounds how quickly a toggle/interval
// change — or a due refresh — is noticed.
const schedulerTick = 1 * time.Minute

// startScheduler launches a single background goroutine that refreshes market
// data on the configured interval when background refresh is enabled. It
// re-reads settings on every wake, so toggle/interval changes take effect
// without a restart. A failed run is logged and retried on the next wake; the
// goroutine never exits on error (per T6 acceptance: scheduler stays alive).
func startScheduler(db *gorm.DB) {
	go func() {
		runScheduledRefresh(db) // catch a due refresh promptly on boot
		ticker := time.NewTicker(schedulerTick)
		defer ticker.Stop()
		for range ticker.C {
			runScheduledRefresh(db)
		}
	}()
}

// runScheduledRefresh refreshes market data if background refresh is enabled and
// the configured interval has elapsed since the last background run. Any panic
// or error is recovered so the next wake still fires.
func runScheduledRefresh(db *gorm.DB) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("scheduler: refresh panic (akan coba lagi slot berikutnya): %v", r)
		}
	}()

	s, err := loadSettings(db)
	if err != nil {
		log.Printf("scheduler: baca settings gagal (akan coba lagi): %v", err)
		return
	}
	if !s.BackgroundRefreshEnabled {
		return
	}

	interval := time.Duration(s.RefreshIntervalHours) * time.Hour
	if interval <= 0 {
		interval = 24 * time.Hour // PRD default: daily
	}
	if !dueForRefresh(s.LastBackgroundRefresh, interval) {
		return
	}

	refreshed, failed, lastUpdate, err := runRefresh(db)
	if err != nil {
		log.Printf("scheduler: refresh gagal (akan coba lagi slot berikutnya): %v", err)
		return
	}
	log.Printf("scheduler: refresh selesai — %d berhasil, %d gagal", refreshed, failed)

	if !lastUpdate.IsZero() {
		t := lastUpdate.UTC()
		if err := db.Model(&AppSettings{}).Where("id = ?", s.ID).
			Update("last_background_refresh", t).Error; err != nil {
			log.Printf("scheduler: update last_background_refresh gagal: %v", err)
		}
	}
}

// dueForRefresh is true when there is no prior run, or the interval has elapsed
// since the last background refresh.
func dueForRefresh(last *time.Time, interval time.Duration) bool {
	if last == nil || last.IsZero() {
		return true
	}
	return time.Since(*last) >= interval
}
