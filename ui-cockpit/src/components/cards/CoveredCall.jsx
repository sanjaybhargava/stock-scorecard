import { fmt } from "../../utils/fmt.js";

function StatBox({ label, value, sub, color }) {
  return (
    <div style={{ flex: 1, textAlign: "center", padding: "12px 8px" }}>
      <div style={{ fontSize: 11, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.3, marginBottom: 4 }}>{label}</div>
      <div style={{ fontSize: 18, fontWeight: 700, color: color || "#0f172a", fontFamily: "'JetBrains Mono', monospace" }}>{value}</div>
      {sub && <div style={{ fontSize: 11, color: "#94a3b8", marginTop: 2 }}>{sub}</div>}
    </div>
  );
}

function AnnualRow({ data, annual }) {
  const maxAbs = Math.max(...annual.map(a => Math.abs(a.net)));
  const barWidth = Math.min((Math.abs(data.net) / maxAbs) * 100, 100);
  const isPositive = data.net >= 0;

  return (
    <div style={{
      display: "grid", gridTemplateColumns: "50px 58px 46px 44px 84px 84px 84px 1fr",
      alignItems: "center", padding: "7px 0",
      borderBottom: "1px solid rgba(148,163,184,0.08)",
      gap: 4,
    }}>
      <span style={{ fontSize: 13, fontWeight: 600, color: "#475569" }}>{data.year}</span>
      <span style={{ fontSize: 12, color: "#64748b", textAlign: "center" }}>{data.months}</span>
      <span style={{ fontSize: 12, color: "#16a34a", textAlign: "center" }}>{data.otm}</span>
      <span style={{ fontSize: 12, color: data.itm > 0 ? "#dc2626" : "#cbd5e1", textAlign: "center" }}>{data.itm}</span>
      <span style={{ fontSize: 12, fontWeight: 500, textAlign: "right", color: "#16a34a" }}>+{fmt(data.gross)}</span>
      <span style={{ fontSize: 12, fontWeight: 500, textAlign: "right", color: data.losses < 0 ? "#dc2626" : "#cbd5e1" }}>
        {data.losses < 0 ? `−${fmt(Math.abs(data.losses))}` : "—"}
      </span>
      <span style={{ fontSize: 13, fontWeight: 700, textAlign: "right", color: isPositive ? "#16a34a" : "#dc2626", paddingRight: 6 }}>
        {isPositive ? "+" : "−"}{fmt(Math.abs(data.net))}
      </span>
      <div style={{ display: "flex", alignItems: "center" }}>
        <div style={{
          height: 5, width: `${barWidth}%`, borderRadius: 3,
          background: isPositive ? "rgba(34,197,94,0.5)" : "rgba(239,68,68,0.5)",
        }} />
      </div>
    </div>
  );
}

function RollRow({ event }) {
  return (
    <div style={{
      display: "grid", gridTemplateColumns: "80px 1fr 90px 90px 90px",
      alignItems: "center", padding: "8px 0",
      borderBottom: "1px solid rgba(148,163,184,0.08)",
      gap: 4,
    }}>
      <span style={{ fontSize: 13, fontWeight: 600, color: "#475569" }}>{event.month}</span>
      <span style={{ fontSize: 12, color: "#64748b" }}>
        ₹{event.strike.toLocaleString()} → closed ₹{event.closed.toLocaleString()}
      </span>
      <span style={{ fontSize: 12, fontWeight: 600, textAlign: "right", color: "#dc2626" }}>
        {fmt(event.taxIfAssigned)}
      </span>
      <span style={{ fontSize: 12, fontWeight: 600, textAlign: "right", color: event.rollCost < 0 ? "#16a34a" : "#e65100" }}>
        {event.rollCost < 0 ? `+${fmt(Math.abs(event.rollCost))}` : fmt(event.rollCost)}
      </span>
      <span style={{ fontSize: 12, fontWeight: 700, textAlign: "right", color: "#16a34a" }}>
        {fmt(event.saved)}
      </span>
    </div>
  );
}

export default function CoveredCall({ strategy, backtest, deficit, annual, itmEvents }) {
  const monthlyIncome = Math.round(backtest.annIncome / 12);
  const totalTaxSaved = itmEvents.reduce((s, e) => s + e.saved, 0);

  return (
    <div style={{ background: "#fff", borderRadius: 12, border: "1px solid #e2e8f0", overflow: "hidden" }}>

      {/* Header */}
      <div style={{ padding: "20px 20px 16px" }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 4 }}>
          <div>
            <span style={{ fontSize: 11, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.5, textTransform: "uppercase" }}>Covered Call Potential</span>
            <h2 style={{ fontSize: 18, fontWeight: 700, color: "#0f172a", margin: "4px 0 0" }}>{strategy.stockName}</h2>
          </div>
          <div style={{ padding: "6px 12px", borderRadius: 8, background: "rgba(99,102,241,0.08)" }}>
            <span style={{ fontSize: 14, fontWeight: 700, color: "#6366f1" }}>{backtest.annYield}%</span>
            <span style={{ fontSize: 12, color: "#94a3b8", marginLeft: 4 }}>net yield</span>
          </div>
        </div>
        <p style={{ fontSize: 13, color: "#64748b", margin: "8px 0 0", lineHeight: 1.5 }}>
          Earn income while holding. Backtested over {backtest.totalMonths} monthly expiry cycles.
        </p>
      </div>

      {/* Current opportunity strip */}
      <div style={{ display: "flex", gap: 1, borderTop: "1px solid #e2e8f0", borderBottom: "1px solid #e2e8f0", background: "#f8fafc" }}>
        <StatBox label="STRIKE" value={`₹${strategy.strikePrice.toLocaleString()}`} sub={`${strategy.strikePct}% OTM`} />
        <StatBox label="PREMIUM" value={`~₹${Math.round(strategy.avgPremiumPerShare)}`} sub="per share/month" />
        <StatBox label="MONTHLY" value={`~₹${Math.round(monthlyIncome / 1000)}K`} sub={`${strategy.lots} lots`} />
        <StatBox label="ANNUAL" value={fmt(backtest.annIncome)} color="#6366f1" sub="net of losses" />
      </div>

      {/* Strategy rules */}
      <div style={{ padding: "16px 20px", borderBottom: "1px solid #e2e8f0" }}>
        <div style={{ fontSize: 12, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.3, textTransform: "uppercase", marginBottom: 10 }}>Strategy rules</div>
        <div style={{ display: "grid", gridTemplateColumns: "auto 1fr", gap: "8px 16px", fontSize: 13, lineHeight: 1.6, color: "#475569" }}>
          <span style={{ fontWeight: 600, color: "#1e293b" }}>Strike</span>
          <span>8% OTM — 93% of monthly expiries never reached this level</span>
          <span style={{ fontWeight: 600, color: "#1e293b" }}>Entry</span>
          <span>Sell when stock is ≥1% above start of expiry period (confirms upward move)</span>
          <span style={{ fontWeight: 600, color: "#1e293b" }}>Hold</span>
          <span>No stop loss — hold to expiry. Intra-month spikes usually reverse.</span>
          <span style={{ fontWeight: 600, color: "#1e293b" }}>If ITM</span>
          <span style={{ color: "#6366f1", fontWeight: 500 }}>Always roll, never get assigned. Roll up to new 8% OTM strike on next month. Avoids triggering LTCG tax.</span>
          <span style={{ fontWeight: 600, color: "#1e293b" }}>Sizing</span>
          <span>28,400 shares (56 lots) until Jan 2026, then 25,400 shares (50 lots)</span>
        </div>
      </div>

      {/* Backtest headline */}
      <div style={{ padding: "16px 20px 12px", borderBottom: "1px solid #e2e8f0" }}>
        <div style={{ fontSize: 12, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.3, textTransform: "uppercase", marginBottom: 12 }}>10-Year Backtest</div>

        {/* Outcome boxes */}
        <div style={{ display: "flex", gap: 1, marginBottom: 16, borderRadius: 8, overflow: "hidden", border: "1px solid #e2e8f0" }}>
          <div style={{ flex: 2, background: "rgba(34,197,94,0.06)", padding: "12px 8px", textAlign: "center" }}>
            <div style={{ fontSize: 24, fontWeight: 700, color: "#16a34a" }}>{backtest.otm}</div>
            <div style={{ fontSize: 12, color: "#64748b" }}>Expired worthless</div>
            <div style={{ fontSize: 11, color: "#16a34a", fontWeight: 500 }}>kept premium</div>
          </div>
          <div style={{ flex: 1, background: "rgba(239,68,68,0.04)", padding: "12px 8px", textAlign: "center" }}>
            <div style={{ fontSize: 24, fontWeight: 700, color: "#dc2626" }}>{backtest.itm}</div>
            <div style={{ fontSize: 12, color: "#64748b" }}>Expired ITM</div>
            <div style={{ fontSize: 11, color: "#6366f1", fontWeight: 500 }}>rolled forward</div>
          </div>
          <div style={{ flex: 1, background: "rgba(148,163,184,0.04)", padding: "12px 8px", textAlign: "center" }}>
            <div style={{ fontSize: 24, fontWeight: 700, color: "#94a3b8" }}>{backtest.skipped}</div>
            <div style={{ fontSize: 12, color: "#64748b" }}>No entry</div>
            <div style={{ fontSize: 11, color: "#94a3b8" }}>skipped</div>
          </div>
        </div>

        {/* Gross / Loss / Net */}
        <div style={{ display: "flex", gap: 1, borderRadius: 8, overflow: "hidden", border: "1px solid #e2e8f0", marginBottom: 16 }}>
          <div style={{ flex: 1, background: "rgba(34,197,94,0.04)", padding: "12px 12px", textAlign: "center" }}>
            <div style={{ fontSize: 11, color: "#16a34a", fontWeight: 600, marginBottom: 2 }}>Premiums kept</div>
            <div style={{ fontSize: 16, fontWeight: 700, color: "#16a34a", fontFamily: "'JetBrains Mono', monospace" }}>+{fmt(backtest.premiumsKept)}</div>
          </div>
          <div style={{ flex: 1, background: "rgba(239,68,68,0.04)", padding: "12px 12px", textAlign: "center" }}>
            <div style={{ fontSize: 11, color: "#dc2626", fontWeight: 600, marginBottom: 2 }}>Roll costs (ITM)</div>
            <div style={{ fontSize: 16, fontWeight: 700, color: "#dc2626", fontFamily: "'JetBrains Mono', monospace" }}>−{fmt(Math.abs(backtest.itmLosses))}</div>
          </div>
          <div style={{ flex: 1, background: "rgba(99,102,241,0.04)", padding: "12px 12px", textAlign: "center" }}>
            <div style={{ fontSize: 11, color: "#6366f1", fontWeight: 600, marginBottom: 2 }}>Net income</div>
            <div style={{ fontSize: 16, fontWeight: 700, color: "#6366f1", fontFamily: "'JetBrains Mono', monospace" }}>+{fmt(backtest.netPnl)}</div>
          </div>
        </div>

        {/* Annual table */}
        <div>
          <div style={{
            display: "grid", gridTemplateColumns: "50px 58px 46px 44px 84px 84px 84px 1fr",
            padding: "0 0 6px", borderBottom: "1px solid rgba(148,163,184,0.2)", gap: 4,
          }}>
            {["YEAR", "TRADED", "OTM", "ITM", "EARNED", "LOST", "NET", ""].map((h, i) => (
              <span key={h || i} style={{
                fontSize: 10, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.3,
                textAlign: [0].includes(i) ? "left" : [1, 2, 3].includes(i) ? "center" : i < 7 ? "right" : "left",
                paddingRight: i === 6 ? 6 : 0,
              }}>{h}</span>
            ))}
          </div>
          {annual.map(a => <AnnualRow key={a.year} data={a} annual={annual} />)}
        </div>
      </div>

      {/* Roll vs Assignment */}
      <div style={{ padding: "16px 20px", borderBottom: "1px solid #e2e8f0" }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 10 }}>
          <div style={{ fontSize: 12, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.3, textTransform: "uppercase" }}>
            Why roll, never get assigned
          </div>
          <div style={{ padding: "4px 10px", borderRadius: 6, background: "rgba(34,197,94,0.08)", fontSize: 12, fontWeight: 600, color: "#16a34a" }}>
            {fmt(totalTaxSaved)} LTCG tax avoided
          </div>
        </div>
        <p style={{ fontSize: 13, color: "#64748b", margin: "0 0 12px", lineHeight: 1.5 }}>
          Assignment triggers LTCG on the full gain from ₹1,352 cost basis. Rolling costs a fraction and keeps your position intact.
        </p>

        <div style={{
          display: "grid", gridTemplateColumns: "80px 1fr 90px 90px 90px",
          padding: "0 0 6px", borderBottom: "1px solid rgba(148,163,184,0.2)", gap: 4,
        }}>
          {["MONTH", "STRIKE → CLOSE", "TAX IF ASSIGNED", "ROLL COST", "SAVED"].map((h, i) => (
            <span key={h} style={{
              fontSize: 10, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.3,
              textAlign: i < 2 ? "left" : "right",
            }}>{h}</span>
          ))}
        </div>
        {itmEvents.map((e, i) => <RollRow key={i} event={e} />)}
      </div>

      {/* Context: deficit recovery */}
      <div style={{ padding: "16px 20px", background: "rgba(239,68,68,0.03)" }}>
        <div style={{ fontSize: 12, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.3, textTransform: "uppercase", marginBottom: 12 }}>
          vs 3% test deficit
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 12 }}>
          <div style={{ flex: 1 }}>
            <div style={{ fontSize: 11, color: "#64748b", marginBottom: 2 }}>Opportunity cost</div>
            <div style={{ fontSize: 18, fontWeight: 700, color: "#dc2626", fontFamily: "'JetBrains Mono', monospace" }}>−{fmt(deficit.threePercentDeficit)}</div>
          </div>
          <div style={{ fontSize: 18, color: "#cbd5e1" }}>→</div>
          <div style={{ flex: 1 }}>
            <div style={{ fontSize: 11, color: "#64748b", marginBottom: 2 }}>CC net income</div>
            <div style={{ fontSize: 18, fontWeight: 700, color: "#6366f1", fontFamily: "'JetBrains Mono', monospace" }}>+{fmt(deficit.recoveredByCC)}</div>
          </div>
          <div style={{ fontSize: 18, color: "#cbd5e1" }}>→</div>
          <div style={{ flex: 1 }}>
            <div style={{ fontSize: 11, color: "#64748b", marginBottom: 2 }}>Still behind</div>
            <div style={{ fontSize: 18, fontWeight: 700, color: "#dc2626", fontFamily: "'JetBrains Mono', monospace" }}>−{fmt(deficit.threePercentDeficit - deficit.recoveredByCC)}</div>
          </div>
        </div>
        <div style={{ height: 8, background: "#fee2e2", borderRadius: 4, overflow: "hidden" }}>
          <div style={{
            height: "100%", width: `${deficit.recoveryPct}%`,
            background: "linear-gradient(90deg, #6366f1, #818cf8)", borderRadius: 4,
          }} />
        </div>
        <div style={{ fontSize: 12, color: "#64748b", marginTop: 6, textAlign: "center" }}>
          Covered calls would have recovered <strong style={{ color: "#6366f1" }}>{deficit.recoveryPct}%</strong> of the opportunity cost
        </div>
      </div>
    </div>
  );
}
