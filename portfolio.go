package main

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Position is a held IHSG stock. Stored in SQLite via GORM.
// Market-dependent fields (current price, verdicts) are computed in later
// slices once MarketData/AnalysisResult exist; T1 only persists cost basis.
type Position struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	Ticker      string    `gorm:"not null" json:"ticker"`
	Shares      int       `gorm:"not null" json:"shares"`
	AvgBuyPrice float64   `gorm:"not null" json:"avg_buy_price"`
	BuyDate     string    `gorm:"not null" json:"buy_date"` // YYYY-MM-DD
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// --- Request DTOs ---

type createPositionRequest struct {
	Ticker      string  `json:"ticker"`
	Shares      int     `json:"shares"`
	AvgBuyPrice float64 `json:"avg_buy_price"`
	BuyDate     string  `json:"buy_date"`
}

// Pointer fields → partial update (PUT). Only provided fields are changed.
type updatePositionRequest struct {
	Ticker      *string  `json:"ticker"`
	Shares      *int     `json:"shares"`
	AvgBuyPrice *float64 `json:"avg_buy_price"`
	BuyDate     *string  `json:"buy_date"`
}

// --- Response DTOs (match docs/api-contract.md shapes) ---

type summaryResponse struct {
	TotalInvestmentIDR float64  `json:"total_investment_idr"`
	CurrentValueIDR    *float64 `json:"current_value_idr"`     // null until T2 market data
	TotalProfitLossIDR *float64 `json:"total_profit_loss_idr"` // null until T2
	TotalProfitLossPct *float64 `json:"total_profit_loss_pct"` // null until T2
	LastMarketUpdate   *string  `json:"last_market_update"`    // null until T2
}

type verdictSet struct {
	Rule         *string `json:"rule"` // null until T2/T3
	AI           *string `json:"ai"`   // null until T3
	Disagreement bool    `json:"disagreement"`
}

type positionResponse struct {
	ID              string   `json:"id"`
	Ticker          string   `json:"ticker"`
	Shares          int      `json:"shares"`
	AvgBuyPrice     float64  `json:"avg_buy_price"`
	BuyDate         string   `json:"buy_date"`
	CurrentPrice    *float64 `json:"current_price"`     // null until T2
	CurrentValueIDR *float64 `json:"current_value_idr"` // null until T2
	ProfitLossIDR   *float64 `json:"profit_loss_idr"`   // null until T2
	ProfitLossPct   *float64 `json:"profit_loss_pct"`   // null until T2
	WeightPct       float64  `json:"weight_pct"`        // cost-basis allocation, computed
	Verdicts        *struct {
		ShortTerm verdictSet `json:"short_term"`
		LongTerm  verdictSet `json:"long_term"`
	} `json:"verdicts"` // null until T2/T3
}

type portfolioResponse struct {
	Summary   summaryResponse    `json:"summary"`
	Positions []positionResponse `json:"positions"`
}

// validatePositionInput checks the trust-boundary inputs. Returns the
// normalized (uppercased, trimmed) ticker.
func validatePositionInput(ticker string, shares int, avg float64, date string) (string, error) {
	t := strings.ToUpper(strings.TrimSpace(ticker))
	if t == "" {
		return "", errors.New("ticker wajib diisi")
	}
	if shares <= 0 {
		return "", errors.New("shares harus lebih dari 0")
	}
	if avg <= 0 {
		return "", errors.New("avg_buy_price harus lebih dari 0")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return "", errors.New("buy_date harus format YYYY-MM-DD")
	}
	return t, nil
}

// GET /api/v1/portfolio — summary (cost basis) + positions with allocation %.
func listPortfolio(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var positions []Position
		if err := db.Order("ticker ASC").Find(&positions).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "gagal memuat portofolio"})
		}

		total := 0.0
		for _, p := range positions {
			total += float64(p.Shares) * p.AvgBuyPrice
		}

		items := make([]positionResponse, 0, len(positions))
		for _, p := range positions {
			cost := float64(p.Shares) * p.AvgBuyPrice
			weight := 0.0
			if total > 0 {
				weight = cost / total * 100
			}
			items = append(items, positionResponse{
				ID:          p.ID,
				Ticker:      p.Ticker,
				Shares:      p.Shares,
				AvgBuyPrice: p.AvgBuyPrice,
				BuyDate:     p.BuyDate,
				WeightPct:   weight,
				// CurrentPrice / Value / P&L / Verdicts stay nil → placeholders until T2.
			})
		}

		return c.JSON(portfolioResponse{
			Summary:   summaryResponse{TotalInvestmentIDR: total},
			Positions: items,
		})
	}
}

// POST /api/v1/portfolio
func createPosition(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req createPositionRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "body tidak valid"})
		}
		ticker, err := validatePositionInput(req.Ticker, req.Shares, req.AvgBuyPrice, req.BuyDate)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}

		p := Position{
			ID:          uuid.NewString(),
			Ticker:      ticker,
			Shares:      req.Shares,
			AvgBuyPrice: req.AvgBuyPrice,
			BuyDate:     req.BuyDate,
		}
		if err := db.Create(&p).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "gagal menyimpan posisi"})
		}
		return c.Status(201).JSON(p)
	}
}

// PUT /api/v1/portfolio/:id — partial update of editable fields.
func updatePosition(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		var p Position
		if err := db.First(&p, "id = ?", id).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "posisi tidak ditemukan"})
		}

		var req updatePositionRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "body tidak valid"})
		}

		// Merge provided fields, then validate the resulting state.
		if req.Ticker != nil || req.Shares != nil || req.AvgBuyPrice != nil || req.BuyDate != nil {
			ticker := p.Ticker
			if req.Ticker != nil {
				ticker = *req.Ticker
			}
			shares := p.Shares
			if req.Shares != nil {
				shares = *req.Shares
			}
			avg := p.AvgBuyPrice
			if req.AvgBuyPrice != nil {
				avg = *req.AvgBuyPrice
			}
			date := p.BuyDate
			if req.BuyDate != nil {
				date = *req.BuyDate
			}
			norm, err := validatePositionInput(ticker, shares, avg, date)
			if err != nil {
				return c.Status(400).JSON(fiber.Map{"error": err.Error()})
			}
			p.Ticker, p.Shares, p.AvgBuyPrice, p.BuyDate = norm, shares, avg, date
		}

		if err := db.Save(&p).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "gagal memperbarui posisi"})
		}
		return c.JSON(p)
	}
}

// DELETE /api/v1/portfolio/:id
func deletePosition(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		res := db.Delete(&Position{}, "id = ?", id)
		if res.Error != nil {
			return c.Status(500).JSON(fiber.Map{"error": "gagal menghapus posisi"})
		}
		if res.RowsAffected == 0 {
			return c.Status(404).JSON(fiber.Map{"error": "posisi tidak ditemukan"})
		}
		return c.SendStatus(204)
	}
}
