import { fmt } from "../utils/fmt.js";

function ClassCard({ label, sublabel, invested, current, count, color, borderColor }) {
  const gain = current - invested;
  const gainPct = invested > 0 ? ((current / invested - 1) * 100).toFixed(1) : 0;
  const isUp = gain >= 0;

  return (
    <div style={{
      flex: 1, padding: "16px 14px", borderRadius: 10,
      background: "#fff", border: `1px solid ${borderColor || "#e2e8f0"}`,
      minWidth: 140,
    }}>
      <div style={{ fontSize: 11, fontWeight: 600, color: color || "#94a3b8", letterSpacing: 0.3, textTransform: "uppercase", marginBottom: 8 }}>
        {label}
      </div>
      <div style={{ fontSize: 18, fontWeight: 700, color: "#0f172a", fontFamily: "'JetBrains Mono', monospace", marginBottom: 4 }}>
        {fmt(current)}
      </div>
      <div style={{ fontSize: 12, color: "#94a3b8", marginBottom: 6 }}>
        invested {fmt(invested)}
      </div>
      <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
        <span style={{ fontSize: 13, fontWeight: 600, color: isUp ? "#16a34a" : "#dc2626" }}>
          {isUp ? "+" : ""}{gainPct}%
        </span>
        {sublabel && <span style={{ fontSize: 11, color: "#94a3b8" }}>{sublabel}</span>}
      </div>
      {count != null && (
        <div style={{ fontSize: 11, color: "#94a3b8", marginTop: 4 }}>
          {count} {count === 1 ? "holding" : "holdings"}
        </div>
      )}
    </div>
  );
}

export default function PortfolioSummary({ data, onNavigate }) {
  const { portfolio, stocks, summary } = data;
  const bc = portfolio.by_class;

  // HUL is in stocks — add HUL invested to stock class
  const stockInvested = bc.stock.invested;
  const stockCurrent = bc.stock.current;
  const stockCount = bc.stock.count;

  return (
    <div>
      {/* Header */}
      <div style={{ marginBottom: 24 }}>
        <div style={{ fontSize: 11, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.5, textTransform: "uppercase", marginBottom: 4 }}>
          {data.client_name ? `${data.client_name}'s Portfolio` : "YOUR PORTFOLIO"}
        </div>
        <div style={{ display: "flex", alignItems: "baseline", gap: 12 }}>
          <h1 style={{ fontSize: 28, fontWeight: 700, color: "#0f172a", margin: 0, fontFamily: "'JetBrains Mono', monospace" }}>
            {fmt(portfolio.total_current)}
          </h1>
          <span style={{ fontSize: 14, color: "#64748b" }}>
            across {stockCount + bc.gold_etf.count + bc.active_mf.count + bc.index_mf.count} holdings
          </span>
        </div>
        <div style={{ fontSize: 13, color: "#94a3b8", marginTop: 4 }}>
          Report date: {data.report_date}
        </div>
      </div>

      {/* Asset class cards */}
      <div style={{ display: "flex", gap: 12, marginBottom: 24, flexWrap: "wrap" }}>
        <ClassCard
          label="Stocks"
          sublabel="3% test"
          invested={stockInvested}
          current={stockCurrent}
          count={stockCount}
          color="#6366f1"
          borderColor="rgba(99,102,241,0.2)"
        />
        <ClassCard
          label="Gold"
          sublabel="3% test"
          invested={bc.gold_etf.invested}
          current={bc.gold_etf.current}
          count={bc.gold_etf.count}
          color="#d97706"
          borderColor="rgba(217,119,6,0.2)"
        />
        <ClassCard
          label="Active MF"
          sublabel="2% test"
          invested={bc.active_mf.invested}
          current={bc.active_mf.current}
          count={bc.active_mf.count}
          color="#0891b2"
          borderColor="rgba(8,145,178,0.2)"
        />
        <ClassCard
          label="Index MF"
          sublabel="no test"
          invested={bc.index_mf.invested}
          current={bc.index_mf.current}
          count={bc.index_mf.count}
          color="#64748b"
          borderColor="rgba(100,116,139,0.2)"
        />
      </div>

      {/* Hero: active bets summary */}
      <div
        onClick={onNavigate}
        style={{
          padding: "20px 24px", borderRadius: 12, cursor: "pointer",
          background: "#fff", border: "1px solid #e2e8f0",
          transition: "box-shadow 0.2s",
        }}
        onMouseEnter={(e) => e.currentTarget.style.boxShadow = "0 4px 12px rgba(0,0,0,0.08)"}
        onMouseLeave={(e) => e.currentTarget.style.boxShadow = "none"}
      >
        <div style={{ fontSize: 12, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.3, textTransform: "uppercase", marginBottom: 14 }}>
          ACTIVE BETS
        </div>

        {/* Summary strip */}
        <div style={{ display: "flex", gap: 1, marginBottom: 16, borderRadius: 8, overflow: "hidden", border: "1px solid #e2e8f0" }}>
          <div style={{ flex: 1, background: "rgba(34,197,94,0.04)", padding: "12px 14px", textAlign: "center" }}>
            <div style={{ fontSize: 12, color: "#16a34a", fontWeight: 600, marginBottom: 2 }}>Passed</div>
            <div style={{ fontSize: 22, fontWeight: 700, color: "#16a34a" }}>{summary.pass_count}</div>
            <div style={{ fontSize: 12, color: "#16a34a", fontWeight: 500 }}>+{fmt(summary.total_surplus)}</div>
          </div>
          <div style={{ flex: 1, background: "rgba(239,68,68,0.04)", padding: "12px 14px", textAlign: "center" }}>
            <div style={{ fontSize: 12, color: "#dc2626", fontWeight: 600, marginBottom: 2 }}>Failed</div>
            <div style={{ fontSize: 22, fontWeight: 700, color: "#dc2626" }}>{summary.fail_count}</div>
            <div style={{ fontSize: 12, color: "#dc2626", fontWeight: 500 }}>{fmt(summary.total_deficit)}</div>
          </div>
          <div style={{ flex: 1, background: "rgba(148,163,184,0.04)", padding: "12px 14px", textAlign: "center" }}>
            <div style={{ fontSize: 12, color: "#64748b", fontWeight: 500, marginBottom: 2 }}>Too Early</div>
            <div style={{ fontSize: 22, fontWeight: 700, color: "#64748b" }}>{summary.too_early_count}</div>
          </div>
        </div>

        {/* Cost of failures hero */}
        <div style={{
          padding: "14px 18px", borderRadius: 8,
          background: "rgba(239,68,68,0.06)", border: "1px solid rgba(239,68,68,0.15)",
          display: "flex", justifyContent: "space-between", alignItems: "center",
        }}>
          <span style={{ fontSize: 14, fontWeight: 500, color: "#475569" }}>Cost of failures</span>
          <span style={{
            fontSize: 22, fontWeight: 700, color: "#dc2626",
            fontFamily: "'JetBrains Mono', monospace",
          }}>
            −{fmt(summary.total_deficit)}
          </span>
        </div>

        <div style={{ textAlign: "center", marginTop: 14 }}>
          <span style={{ fontSize: 13, color: "#6366f1", fontWeight: 500, cursor: "pointer" }}>
            See detailed results →
          </span>
        </div>
      </div>
    </div>
  );
}
