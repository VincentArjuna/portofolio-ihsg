package main

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// AppSettings is the single-row settings record (id always singletonSettingsID).
// Per docs/domain.md + docs/api-contract.md: background refresh toggle, its
// interval, the last scheduled run timestamp, and the Hermes CLI path.
type AppSettings struct {
	ID                       uint       `gorm:"primaryKey" json:"-"`
	BackgroundRefreshEnabled bool       `gorm:"not null;default:false" json:"background_refresh_enabled"`
	RefreshIntervalHours     int        `gorm:"not null;default:24" json:"refresh_interval_hours"`
	LastBackgroundRefresh    *time.Time `json:"last_background_refresh"`
	HermesExecutable         string     `gorm:"not null;default:hermes" json:"hermes_executable"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
}

const singletonSettingsID uint = 1

// loadSettings returns the singleton settings row, creating defaults on first
// access: background refresh OFF, daily interval (PRD default), `hermes` on PATH.
func loadSettings(db *gorm.DB) (AppSettings, error) {
	var s AppSettings
	if err := db.First(&s, singletonSettingsID).Error; err == nil {
		// Older rows (created before T4) have an empty HermesExecutable.
		if s.HermesExecutable == "" {
			s.HermesExecutable = "hermes"
		}
		return s, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return AppSettings{}, err
	}
	s = AppSettings{
		ID:                       singletonSettingsID,
		BackgroundRefreshEnabled: false,
		RefreshIntervalHours:     24,
		HermesExecutable:         "hermes",
	}
	if err := db.Create(&s).Error; err != nil {
		return AppSettings{}, err
	}
	return s, nil
}

type settingsResponse struct {
	BackgroundRefreshEnabled bool       `json:"background_refresh_enabled"`
	RefreshIntervalHours     int        `json:"refresh_interval_hours"`
	LastBackgroundRefresh    *time.Time `json:"last_background_refresh"`
	HermesExecutable         string     `json:"hermes_executable"`
}

// Pointer fields → partial update (PUT). Only provided fields change.
type updateSettingsRequest struct {
	BackgroundRefreshEnabled *bool   `json:"background_refresh_enabled"`
	RefreshIntervalHours     *int    `json:"refresh_interval_hours"`
	HermesExecutable         *string `json:"hermes_executable"`
}

// GET /api/v1/settings
func getSettings(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		s, err := loadSettings(db)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "gagal memuat pengaturan"})
		}
		return c.JSON(settingsResponse{
			BackgroundRefreshEnabled: s.BackgroundRefreshEnabled,
			RefreshIntervalHours:     s.RefreshIntervalHours,
			LastBackgroundRefresh:    s.LastBackgroundRefresh,
			HermesExecutable:         s.HermesExecutable,
		})
	}
}

// PUT /api/v1/settings — partial update of refresh settings.
func updateSettings(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		s, err := loadSettings(db)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "gagal memuat pengaturan"})
		}

		var req updateSettingsRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "body tidak valid"})
		}

		if req.BackgroundRefreshEnabled != nil {
			s.BackgroundRefreshEnabled = *req.BackgroundRefreshEnabled
		}
		if req.RefreshIntervalHours != nil {
			if *req.RefreshIntervalHours < 1 {
				return c.Status(400).JSON(fiber.Map{"error": "refresh_interval_hours minimal 1"})
			}
			s.RefreshIntervalHours = *req.RefreshIntervalHours
		}
		if req.HermesExecutable != nil {
			he := strings.TrimSpace(*req.HermesExecutable)
			if he == "" {
				return c.Status(400).JSON(fiber.Map{"error": "hermes_executable wajib diisi"})
			}
			s.HermesExecutable = he
		}

		if err := db.Save(&s).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "gagal menyimpan pengaturan"})
		}
		return c.JSON(settingsResponse{
			BackgroundRefreshEnabled: s.BackgroundRefreshEnabled,
			RefreshIntervalHours:     s.RefreshIntervalHours,
			LastBackgroundRefresh:    s.LastBackgroundRefresh,
			HermesExecutable:         s.HermesExecutable,
		})
	}
}
