// API client for the Go backend. Shapes mirror docs/api-contract.md.

export interface VerdictSet {
  rule: string | null;
  ai: string | null;
  disagreement: boolean;
}

export interface Position {
  id: string;
  ticker: string;
  shares: number;
  avg_buy_price: number;
  buy_date: string;
  current_price: number | null; // null until T2 (market data)
  current_value_idr: number | null;
  profit_loss_idr: number | null;
  profit_loss_pct: number | null;
  weight_pct: number; // cost-basis allocation, always computed
  verdicts: { short_term: VerdictSet; long_term: VerdictSet } | null;
}

export interface Summary {
  total_investment_idr: number;
  current_value_idr: number | null;
  total_profit_loss_idr: number | null;
  total_profit_loss_pct: number | null;
  last_market_update: string | null;
}

export interface Portfolio {
  summary: Summary;
  positions: Position[];
}

export interface RefreshResult {
  refreshed_count: number;
  updated_at: string;
  failed: number;
}

export interface PositionInput {
  ticker: string;
  shares: number;
  avg_buy_price: number;
  buy_date: string;
}

export interface Settings {
  background_refresh_enabled: boolean;
  refresh_interval_hours: number;
  last_background_refresh: string | null;
  hermes_executable: string;
}

// --- Stock detail (T3) — shapes mirror GET /stocks/:ticker ---

export interface MarketDataDetail {
  last_price: number;
  prev_close: number;
  pe_ratio: number;
  pbv_ratio: number;
  roe: number;
  der: number;
  rev_growth: number;
  net_margin: number;
  ma20: number;
  ma50: number;
  ma200: number;
  updated_at: string;
}

export interface RuleDetail {
  verdict: string; // BUY | HOLD | SELL
  score: number; // 0-100
  breakdown: Record<string, number>;
}

export interface StockHorizon {
  horizon: string; // "6-12 Bulan" | "3-5 Tahun"
  rule: RuleDetail | null;
  ai: {
    verdict?: string;
    explanation?: string;
    confidence?: number;
    risk_factors?: string[];
    updated_at?: string;
  } | null; // null until T4
  disagreement: boolean; // true when rule + ai verdicts both present and differ
  risk_flags: string[];
}

export interface StockDetail {
  ticker: string;
  company_name: string;
  needs_refresh: boolean;
  market_data: MarketDataDetail;
  short_term: StockHorizon;
  long_term: StockHorizon;
}

export type SettingsInput = Partial<
  Pick<Settings, "background_refresh_enabled" | "refresh_interval_hours" | "hermes_executable">
>;

// --- AI analysis (T4) — POST /stocks/:ticker/ai-analyze ---

export interface AIAnalyzeResponse {
  status: "done" | "cached" | "unavailable" | "error";
  message?: string;
  detail?: StockDetail;
}

const BASE = "/api/v1";

async function asJson<T>(r: Response): Promise<T> {
  if (!r.ok) {
    let msg = `permintaan gagal (${r.status})`;
    try {
      const body = await r.json();
      if (body?.error) msg = body.error;
    } catch {
      /* keep default */
    }
    throw new Error(msg);
  }
  return r.headers.get("content-type")?.includes("application/json")
    ? (r.json() as Promise<T>)
    : (undefined as unknown as T);
}

export const fetchPortfolio = () =>
  fetch(`${BASE}/portfolio`).then(asJson<Portfolio>);

export const refreshMarketData = () =>
  fetch(`${BASE}/market-data/refresh`, { method: "POST" }).then(
    asJson<RefreshResult>,
  );

export const createPosition = (input: PositionInput) =>
  fetch(`${BASE}/portfolio`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  }).then(asJson<Position>);

export const updatePosition = (id: string, input: Partial<PositionInput>) =>
  fetch(`${BASE}/portfolio/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  }).then(asJson<Position>);

export const deletePosition = (id: string) =>
  fetch(`${BASE}/portfolio/${id}`, { method: "DELETE" }).then((r) => {
    if (!r.ok && r.status !== 204) throw new Error(`gagal menghapus (${r.status})`);
  });

export const fetchSettings = () =>
  fetch(`${BASE}/settings`).then(asJson<Settings>);

export const saveSettings = (input: SettingsInput) =>
  fetch(`${BASE}/settings`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  }).then(asJson<Settings>);

export const fetchStockDetail = (ticker: string) =>
  fetch(`${BASE}/stocks/${encodeURIComponent(ticker)}`).then(asJson<StockDetail>);

export const analyzeStockAI = (ticker: string) =>
  fetch(`${BASE}/stocks/${encodeURIComponent(ticker)}/ai-analyze`, {
    method: "POST",
  }).then(asJson<AIAnalyzeResponse>);

// --- Opportunities (T5) — LQ45/Kompas100 ranked discovery ---

export type IndexMembership = "LQ45" | "KOMPAS100" | "BOTH";

export interface Opportunity {
  ticker: string;
  company_name: string;
  index_membership: IndexMembership;
  sector: string;
  last_price: number;
  short_term_rule: string; // BUY | HOLD | SELL
  short_term_score: number; // 0-100
  long_term_rule: string;
  long_term_score: number;
  roe: number;
  per: number;
  risk_flags: string[];
  short_term_breakdown: Record<string, number>;
  long_term_breakdown: Record<string, number>;
}

export interface OpportunityList {
  opportunities: Opportunity[];
}

/** Custom-ticker lookup result; `illiquid` is true outside LQ45/Kompas100. */
export interface LookupResult {
  ticker: string;
  name?: string;
  index_membership?: string;
  sector?: string;
  in_universe: boolean;
  illiquid: boolean;
}

export const fetchOpportunities = (filter = "", minVerdict = "", q = "") => {
  const params = new URLSearchParams();
  if (filter) params.set("filter", filter);
  if (minVerdict) params.set("min_verdict", minVerdict);
  if (q) params.set("q", q);
  const qs = params.toString();
  return fetch(`${BASE}/opportunities${qs ? `?${qs}` : ""}`).then(
    asJson<OpportunityList>,
  );
};

export const lookupTicker = (q: string) =>
  fetch(`${BASE}/opportunities/lookup?q=${encodeURIComponent(q)}`).then(
    asJson<LookupResult>,
  );

export const refreshOpportunities = () =>
  fetch(`${BASE}/opportunities/refresh`, { method: "POST" }).then(
    asJson<RefreshResult>,
  );
