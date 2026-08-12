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

export interface PositionInput {
  ticker: string;
  shares: number;
  avg_buy_price: number;
  buy_date: string;
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
