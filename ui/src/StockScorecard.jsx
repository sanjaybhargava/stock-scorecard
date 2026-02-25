import { useState, useMemo, useEffect } from "react";

const fmt = (n) => {
  if (n === 0) return "₹0";
  const abs = Math.abs(n);
  if (abs >= 10000000) return `${n < 0 ? "-" : ""}₹${(abs / 10000000).toFixed(2)}Cr`;
  if (abs >= 100000) return `${n < 0 ? "-" : ""}₹${(abs / 100000).toFixed(2)}L`;
  return `${n < 0 ? "-" : ""}₹${abs.toLocaleString("en-IN")}`;
};

const AlphaChip = ({ value, size = "sm" }) => {
  const isPos = value >= 0;
  const sizes = {
    sm: "text-xs px-2 py-0.5",
    md: "text-sm px-3 py-1 font-semibold",
    lg: "text-lg px-4 py-1.5 font-bold",
    xl: "text-2xl px-5 py-2 font-bold",
  };
  return (
    <span className={`inline-flex items-center rounded-full ${sizes[size]} ${isPos ? "bg-emerald-100 text-emerald-800" : "bg-red-100 text-red-800"}`}>
      {isPos ? "▲" : "▼"} {fmt(Math.abs(value))}
    </span>
  );
};

const PassFail = ({ pass }) => (
  <span className={`inline-flex items-center gap-1 text-xs font-bold px-2 py-0.5 rounded ${pass ? "bg-emerald-600 text-white" : "bg-red-600 text-white"}`}>
    {pass ? "✓ PASS" : "✗ FAIL"}
  </span>
);

// ── Level 3: Trade Detail ──
const TradeDetail = ({ ticker, fy, type, trades, onBack }) => {
  const totalInvested = trades.reduce((s, t) => s + t.invested, 0);
  const totalGL = trades.reduce((s, t) => s + t.equityGL + t.optionIncome + t.dividend, 0);
  const totalNifty = trades.reduce((s, t) => s + t.niftyReturn, 0);
  const alpha = totalGL - totalNifty;

  return (
    <div>
      <button onClick={onBack} className="text-sm text-slate-500 hover:text-slate-800 mb-4 flex items-center gap-1">
        ← Back to {fy} {type}
      </button>
      <div className="flex items-center gap-3 mb-6">
        <h2 className="text-xl font-bold text-slate-900 tracking-tight">{ticker}</h2>
        <span className="text-sm text-slate-500">{fy} · {type}</span>
        <PassFail pass={alpha >= 0} />
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-slate-800 text-white">
              {["Buy Date", "Sell Date", "Days", "Qty", "Invested", "Equity G/L", "Nifty Return", "Alpha"].map(h => (
                <th key={h} className="px-3 py-2 text-right first:text-left font-medium">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {trades.map((t, i) => {
              const tradeAlpha = t.equityGL - t.niftyReturn;
              return (
                <tr key={i} className={i % 2 === 0 ? "bg-slate-50" : "bg-white"}>
                  <td className="px-3 py-2 text-left font-mono text-xs">{t.buyDate}</td>
                  <td className="px-3 py-2 text-right font-mono text-xs">{t.sellDate}</td>
                  <td className="px-3 py-2 text-right">{t.holdDays}</td>
                  <td className="px-3 py-2 text-right">{t.quantity}</td>
                  <td className="px-3 py-2 text-right">{fmt(t.invested)}</td>
                  <td className={`px-3 py-2 text-right ${t.equityGL >= 0 ? "text-emerald-700" : "text-red-700"}`}>{fmt(t.equityGL)}</td>
                  <td className="px-3 py-2 text-right text-slate-600">{fmt(t.niftyReturn)}</td>
                  <td className={`px-3 py-2 text-right font-semibold ${tradeAlpha >= 0 ? "text-emerald-700" : "text-red-700"}`}>{fmt(tradeAlpha)}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      <div className="mt-6 grid grid-cols-3 gap-4">
        <div className="bg-slate-50 rounded-lg p-4 text-center">
          <div className="text-xs text-slate-500 mb-1">My Return</div>
          <div className={`text-lg font-bold ${totalGL >= 0 ? "text-emerald-700" : "text-red-700"}`}>{fmt(totalGL)}</div>
        </div>
        <div className="bg-slate-50 rounded-lg p-4 text-center">
          <div className="text-xs text-slate-500 mb-1">NIFTY 500 Return</div>
          <div className="text-lg font-bold text-slate-700">{fmt(totalNifty)}</div>
        </div>
        <div className={`rounded-lg p-4 text-center ${alpha >= 0 ? "bg-emerald-50" : "bg-red-50"}`}>
          <div className="text-xs text-slate-500 mb-1">Alpha</div>
          <AlphaChip value={alpha} size="md" />
        </div>
      </div>
    </div>
  );
};

// ── Level 2: FY Detail (ticker list) ──
const FYDetail = ({ fy, type, trades, onBack, onDrill }) => {
  const byTicker = {};
  trades.forEach(t => {
    if (!byTicker[t.ticker]) byTicker[t.ticker] = { invested: 0, totalGL: 0, niftyReturn: 0, count: 0 };
    byTicker[t.ticker].invested += t.invested;
    byTicker[t.ticker].totalGL += t.equityGL + t.optionIncome + t.dividend;
    byTicker[t.ticker].niftyReturn += t.niftyReturn;
    byTicker[t.ticker].count += 1;
  });

  const tickers = Object.entries(byTicker)
    .map(([ticker, d]) => ({ ticker, ...d, alpha: d.totalGL - d.niftyReturn }))
    .sort((a, b) => b.alpha - a.alpha);

  const passes = tickers.filter(t => t.alpha >= 0);
  const fails = tickers.filter(t => t.alpha < 0);

  return (
    <div>
      <button onClick={onBack} className="text-sm text-slate-500 hover:text-slate-800 mb-4 flex items-center gap-1">
        ← Back to Scorecard
      </button>
      <div className="flex items-center gap-3 mb-6">
        <h2 className="text-xl font-bold text-slate-900 tracking-tight">{fy}</h2>
        <span className={`text-sm font-semibold px-2 py-0.5 rounded ${type === "Long" ? "bg-blue-100 text-blue-800" : "bg-amber-100 text-amber-800"}`}>{type}</span>
        <span className="text-sm text-slate-500">{tickers.length} tickers</span>
      </div>

      {[{ label: "Passes", data: passes, color: "emerald" }, { label: "Fails", data: fails, color: "red" }].map(({ label, data, color }) => (
        data.length > 0 && (
          <div key={label} className="mb-6">
            <div className={`text-xs font-bold uppercase tracking-wider mb-2 ${color === "emerald" ? "text-emerald-700" : "text-red-700"}`}>
              {label === "Passes" ? "✓" : "✗"} {label} — {data.length} stock{data.length > 1 ? "s" : ""}
            </div>
            <div className="space-y-1">
              {data.map(t => (
                <button
                  key={t.ticker}
                  onClick={() => onDrill(t.ticker)}
                  className={`w-full flex items-center justify-between px-4 py-3 rounded-lg hover:shadow-md transition-all cursor-pointer ${color === "emerald" ? "bg-emerald-50 hover:bg-emerald-100" : "bg-red-50 hover:bg-red-100"}`}
                >
                  <div className="flex items-center gap-3">
                    <span className="font-bold text-slate-900 w-28 text-left">{t.ticker}</span>
                    <span className="text-xs text-slate-500">{t.count} lot{t.count > 1 ? "s" : ""} · {fmt(t.invested)}</span>
                  </div>
                  <div className="flex items-center gap-4">
                    <div className="text-right">
                      <div className="text-xs text-slate-400">G/L</div>
                      <div className={`text-sm font-semibold ${t.totalGL >= 0 ? "text-emerald-700" : "text-red-700"}`}>{fmt(t.totalGL)}</div>
                    </div>
                    <AlphaChip value={t.alpha} size="sm" />
                    <span className="text-slate-400 text-sm">→</span>
                  </div>
                </button>
              ))}
            </div>
          </div>
        )
      ))}
    </div>
  );
};

// ── Map JSON snake_case to camelCase ──
function mapTrade(t) {
  return {
    fy: t.fy,
    type: t.type,
    ticker: t.ticker,
    buyDate: t.buy_date,
    sellDate: t.sell_date,
    holdDays: t.hold_days,
    quantity: t.quantity,
    invested: t.invested,
    equityGL: t.equity_gl,
    optionIncome: 0,
    dividend: 0,
    niftyReturn: t.nifty_return,
    alpha: t.alpha,
  };
}

// ── Level 1: Scorecard ──
export default function StockScorecard() {
  const [view, setView] = useState({ level: 1 });
  const [rawData, setRawData] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    fetch(import.meta.env.BASE_URL + "scorecard.json")
      .then(r => { if (!r.ok) throw new Error(`HTTP ${r.status}`); return r.json(); })
      .then(setRawData)
      .catch(e => setError(e.message));
  }, []);

  const TRADES = useMemo(() => {
    if (!rawData) return [];
    return rawData.trades.map(mapTrade);
  }, [rawData]);

  const openPositions = useMemo(() => {
    if (!rawData) return [];
    return rawData.open_positions.map(o => ({
      ticker: o.ticker,
      buyDate: o.buy_date,
      quantity: o.quantity,
      buyPrice: o.buy_price,
      invested: o.invested,
    }));
  }, [rawData]);

  const warnings = useMemo(() => {
    if (!rawData) return [];
    return rawData.warnings || [];
  }, [rawData]);

  const summary = useMemo(() => {
    const groups = {};
    TRADES.forEach(t => {
      const key = `${t.fy}|${t.type}`;
      if (!groups[key]) groups[key] = { fy: t.fy, type: t.type, trades: 0, invested: 0, myReturn: 0, niftyReturn: 0 };
      groups[key].trades += 1;
      groups[key].invested += t.invested;
      groups[key].myReturn += t.equityGL + t.optionIncome + t.dividend;
      groups[key].niftyReturn += t.niftyReturn;
    });
    return Object.values(groups)
      .map(g => ({ ...g, alpha: g.myReturn - g.niftyReturn }))
      .sort((a, b) => b.fy.localeCompare(a.fy) || a.type.localeCompare(b.type));
  }, [TRADES]);

  const tickerRankings = useMemo(() => {
    const byTicker = {};
    TRADES.forEach(t => {
      if (!byTicker[t.ticker]) byTicker[t.ticker] = { invested: 0, gl: 0, niftyReturn: 0, trades: 0 };
      byTicker[t.ticker].invested += t.invested;
      byTicker[t.ticker].gl += t.equityGL;
      byTicker[t.ticker].niftyReturn += t.niftyReturn;
      byTicker[t.ticker].trades += 1;
    });
    const all = Object.entries(byTicker).map(([ticker, d]) => ({
      ticker, ...d, alpha: d.gl - d.niftyReturn,
    }));
    const winners = all.filter(t => t.alpha >= 0).sort((a, b) => b.alpha - a.alpha);
    const losers = all.filter(t => t.alpha < 0).sort((a, b) => a.alpha - b.alpha);
    return { winners, losers };
  }, [TRADES]);

  const totals = useMemo(() => {
    const all = { trades: 0, invested: 0, myReturn: 0, niftyReturn: 0 };
    const passes = { count: 0, alpha: 0 };
    const fails = { count: 0, alpha: 0 };

    const tickerAlphas = {};
    TRADES.forEach(t => {
      const key = `${t.fy}|${t.type}|${t.ticker}`;
      if (!tickerAlphas[key]) tickerAlphas[key] = { gl: 0, nifty: 0 };
      tickerAlphas[key].gl += t.equityGL + t.optionIncome + t.dividend;
      tickerAlphas[key].nifty += t.niftyReturn;
    });
    Object.values(tickerAlphas).forEach(({ gl, nifty }) => {
      const a = gl - nifty;
      if (a >= 0) { passes.count++; passes.alpha += a; }
      else { fails.count++; fails.alpha += a; }
    });

    summary.forEach(s => {
      all.trades += s.trades;
      all.invested += s.invested;
      all.myReturn += s.myReturn;
      all.niftyReturn += s.niftyReturn;
    });
    all.alpha = all.myReturn - all.niftyReturn;
    const total = passes.count + fails.count;
    return { all, passes, fails, winRate: total > 0 ? Math.round((passes.count / total) * 100) : 0 };
  }, [TRADES, summary]);

  if (error) return <div className="min-h-screen bg-slate-100 flex items-center justify-center"><div className="text-red-600">Failed to load scorecard: {error}</div></div>;
  if (!rawData) return <div className="min-h-screen bg-slate-100 flex items-center justify-center"><div className="text-slate-500">Loading scorecard…</div></div>;

  if (view.level === 3) {
    const trades = TRADES.filter(t => t.fy === view.fy && t.type === view.type && t.ticker === view.ticker);
    return (
      <div className="min-h-screen bg-slate-100 p-4">
        <div className="max-w-4xl mx-auto bg-white rounded-xl shadow-sm p-6">
          <TradeDetail
            ticker={view.ticker} fy={view.fy} type={view.type} trades={trades}
            onBack={() => setView({ level: 2, fy: view.fy, type: view.type })}
          />
        </div>
      </div>
    );
  }

  if (view.level === 2) {
    const trades = TRADES.filter(t => t.fy === view.fy && t.type === view.type);
    return (
      <div className="min-h-screen bg-slate-100 p-4">
        <div className="max-w-4xl mx-auto bg-white rounded-xl shadow-sm p-6">
          <FYDetail
            fy={view.fy} type={view.type} trades={trades}
            onBack={() => setView({ level: 1 })}
            onDrill={(ticker) => setView({ level: 3, fy: view.fy, type: view.type, ticker })}
          />
        </div>
      </div>
    );
  }

  // ── Level 1: The Scorecard ──
  const netAlpha = totals.all.alpha;
  const isGood = netAlpha >= 0;

  return (
    <div className="min-h-screen bg-slate-100 p-4">
      <div className="max-w-4xl mx-auto space-y-4">

        {/* Verdict Card — THE answer */}
        <div className={`rounded-xl p-6 shadow-sm ${isGood ? "bg-emerald-900" : "bg-red-900"}`}>
          <div className="text-center">
            <div className="text-xs font-medium text-white/40 uppercase tracking-widest mb-1">
              Realized Trades · {totals.all.trades} trades
            </div>
            <div className={`text-3xl font-bold mt-2 ${totals.all.myReturn >= 0 ? "text-emerald-300" : "text-red-300"}`}>
              You made {fmt(totals.all.myReturn)}
            </div>
            <div className="text-white/70 text-sm mt-1 mb-3">
              {totals.all.myReturn >= totals.all.niftyReturn
                ? `NIFTY 500 would have made ${fmt(totals.all.niftyReturn)} — you beat the market`
                : `but NIFTY 500 would have made ${fmt(totals.all.niftyReturn)} on the same capital`}
            </div>
            <div className="border-t border-white/10 pt-3 mb-1">
              <div className="text-xs font-medium text-white/40 uppercase tracking-widest mb-1">
                Net Alpha vs NIFTY 500
              </div>
              <div className={`text-4xl font-bold mb-3 ${isGood ? "text-emerald-300" : "text-red-300"}`}>
                {isGood ? "▲" : "▼"} {fmt(Math.abs(netAlpha))}
              </div>
            </div>
            <div className="flex justify-center gap-6 text-sm">
              <div className="text-center">
                <div className="text-white/50">Win Rate</div>
                <div className="text-white font-bold text-lg">{totals.winRate}%</div>
              </div>
              <div className="text-center">
                <div className="text-white/50">Pass Alpha</div>
                <div className="text-emerald-400 font-bold">{fmt(totals.passes.alpha)}</div>
                <div className="text-white/40 text-xs">{totals.passes.count} stocks</div>
              </div>
              <div className="text-center">
                <div className="text-white/50">Fail Alpha</div>
                <div className="text-red-400 font-bold">{fmt(totals.fails.alpha)}</div>
                <div className="text-white/40 text-xs">{totals.fails.count} stocks</div>
              </div>
              <div className="text-center">
                <div className="text-white/50">Deployed</div>
                <div className="text-white font-bold">{fmt(totals.all.invested)}</div>
                <div className="text-white/40 text-xs">{totals.all.trades} trades</div>
              </div>
            </div>
          </div>
        </div>

        {/* FY Breakdown */}
        <div className="bg-white rounded-xl shadow-sm overflow-hidden">
          <div className="px-4 py-3 bg-slate-800">
            <h2 className="text-sm font-semibold text-white uppercase tracking-wider">By Financial Year</h2>
          </div>
          <div className="divide-y divide-slate-100">
            {summary.map((s, i) => {
              const isPass = s.alpha >= 0;
              const isFirstOfFY = i === 0 || summary[i - 1].fy !== s.fy;
              return (
                <button
                  key={`${s.fy}-${s.type}`}
                  onClick={() => setView({ level: 2, fy: s.fy, type: s.type })}
                  className="w-full flex items-center justify-between px-4 py-3 hover:bg-slate-50 transition-colors cursor-pointer"
                >
                  <div className="flex items-center gap-3">
                    <span className={`font-mono text-sm w-24 text-left ${isFirstOfFY ? "font-bold text-slate-900" : "text-slate-400"}`}>
                      {isFirstOfFY ? s.fy : ""}
                    </span>
                    <span className={`text-xs font-semibold px-2 py-0.5 rounded ${s.type === "Long" ? "bg-blue-100 text-blue-700" : "bg-amber-100 text-amber-700"}`}>
                      {s.type}
                    </span>
                    <span className="text-xs text-slate-400">{s.trades} trades · {fmt(s.invested)}</span>
                  </div>
                  <div className="flex items-center gap-3">
                    <div className="text-right hidden sm:block">
                      <div className="text-xs text-slate-400">Me: {fmt(s.myReturn)}</div>
                      <div className="text-xs text-slate-400">N500: {fmt(s.niftyReturn)}</div>
                    </div>
                    <AlphaChip value={s.alpha} size="sm" />
                    <PassFail pass={isPass} />
                    <span className="text-slate-300 text-sm">→</span>
                  </div>
                </button>
              );
            })}
          </div>

          {/* All Years Total */}
          <div className="px-4 py-3 bg-slate-100 flex items-center justify-between border-t-2 border-slate-300">
            <div className="flex items-center gap-3">
              <span className="font-bold text-slate-900 w-24">ALL</span>
              <span className="text-xs text-slate-500">{totals.all.trades} trades · {fmt(totals.all.invested)}</span>
            </div>
            <div className="flex items-center gap-3">
              <AlphaChip value={totals.all.alpha} size="md" />
            </div>
          </div>
        </div>

        {/* Winners & Losers */}
        <div className="bg-white rounded-xl shadow-sm overflow-hidden">
          <div className="px-4 py-3 bg-slate-800">
            <h2 className="text-sm font-semibold text-white uppercase tracking-wider">Winners & Losers — By Ticker</h2>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-0 md:divide-x divide-slate-200">
            {/* Winners */}
            <div>
              <div className="px-4 py-2 bg-emerald-50 border-b border-emerald-200">
                <span className="text-xs font-bold uppercase tracking-wider text-emerald-700">
                  ✓ Winners — {tickerRankings.winners.length} stocks
                </span>
              </div>
              <div className="divide-y divide-slate-100">
                {tickerRankings.winners.map((t, i) => (
                  <div key={t.ticker} className="px-4 py-2 flex items-center justify-between hover:bg-emerald-50/50">
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-slate-400 w-5 text-right">{i + 1}.</span>
                      <span className="font-semibold text-slate-900 text-sm w-28">{t.ticker}</span>
                      <span className="text-xs text-slate-400">{t.trades} trades</span>
                    </div>
                    <div className="flex items-center gap-3">
                      <div className="text-right hidden sm:block">
                        <div className="text-xs text-slate-400">G/L {fmt(t.gl)}</div>
                      </div>
                      <AlphaChip value={t.alpha} size="sm" />
                    </div>
                  </div>
                ))}
              </div>
            </div>
            {/* Losers */}
            <div>
              <div className="px-4 py-2 bg-red-50 border-b border-red-200">
                <span className="text-xs font-bold uppercase tracking-wider text-red-700">
                  ✗ Losers — {tickerRankings.losers.length} stocks
                </span>
              </div>
              <div className="divide-y divide-slate-100">
                {tickerRankings.losers.map((t, i) => (
                  <div key={t.ticker} className="px-4 py-2 flex items-center justify-between hover:bg-red-50/50">
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-slate-400 w-5 text-right">{i + 1}.</span>
                      <span className="font-semibold text-slate-900 text-sm w-28">{t.ticker}</span>
                      <span className="text-xs text-slate-400">{t.trades} trades</span>
                    </div>
                    <div className="flex items-center gap-3">
                      <div className="text-right hidden sm:block">
                        <div className="text-xs text-slate-400">G/L {fmt(t.gl)}</div>
                      </div>
                      <AlphaChip value={t.alpha} size="sm" />
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>

        {/* Open Positions */}
        {openPositions.length > 0 && (
          <div className="bg-white rounded-xl shadow-sm overflow-hidden">
            <div className="px-4 py-3 bg-blue-800">
              <h2 className="text-sm font-semibold text-white uppercase tracking-wider">Open Positions — Still Held</h2>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="bg-slate-50 border-b border-slate-200">
                    {["Ticker", "Buy Date", "Qty", "Buy Price", "Invested"].map(h => (
                      <th key={h} className="px-4 py-2 text-right first:text-left text-xs font-semibold text-slate-500 uppercase">{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {openPositions.map((o, i) => (
                    <tr key={i} className={i % 2 === 0 ? "bg-white" : "bg-slate-50"}>
                      <td className="px-4 py-2 text-left font-bold text-slate-900">{o.ticker}</td>
                      <td className="px-4 py-2 text-right font-mono text-xs text-slate-600">{o.buyDate}</td>
                      <td className="px-4 py-2 text-right">{o.quantity.toLocaleString("en-IN")}</td>
                      <td className="px-4 py-2 text-right text-slate-600">{fmt(Math.round(o.buyPrice))}</td>
                      <td className="px-4 py-2 text-right font-semibold">{fmt(o.invested)}</td>
                    </tr>
                  ))}
                </tbody>
                <tfoot>
                  <tr className="bg-slate-100 border-t-2 border-slate-300">
                    <td colSpan={4} className="px-4 py-2 text-left font-bold text-slate-900">Total</td>
                    <td className="px-4 py-2 text-right font-bold">{fmt(openPositions.reduce((s, o) => s + o.invested, 0))}</td>
                  </tr>
                </tfoot>
              </table>
            </div>
          </div>
        )}

        {/* Warnings */}
        {warnings.length > 0 && (
          <div className="bg-white rounded-xl shadow-sm overflow-hidden">
            <div className="px-4 py-3 bg-amber-700">
              <h2 className="text-sm font-semibold text-white uppercase tracking-wider">Warnings — Unmatched Sells</h2>
            </div>
            <div className="divide-y divide-slate-100">
              {warnings.map((w, i) => (
                <div key={i} className="px-4 py-3 flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <span className="font-bold text-slate-900 w-28">{w.ticker}</span>
                    <span className="text-xs text-slate-500">{w.sell_date}</span>
                  </div>
                  <div className="text-xs text-amber-800 bg-amber-50 px-3 py-1 rounded">
                    {w.unmatched_shares} of {w.total_shares} shares unmatched (pre-account holding)
                  </div>
                </div>
              ))}
            </div>
            <div className="px-4 py-2 bg-amber-50 text-xs text-amber-700 border-t border-amber-200">
              These shares were likely bought before the earliest tradebook (Aug 2019). The sell is recorded but cannot be matched to a buy.
            </div>
          </div>
        )}

      </div>
    </div>
  );
}
