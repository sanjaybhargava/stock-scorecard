import { useState } from "react";
import { fmt } from "../utils/fmt.js";

function Section({ title, badge, badgeColor, count, amount, amountLabel, defaultOpen, children }) {
  const [open, setOpen] = useState(defaultOpen || false);
  const bgMap = { green: "rgba(34,197,94,0.06)", red: "rgba(239,68,68,0.06)", gray: "rgba(148,163,184,0.06)", indigo: "rgba(99,102,241,0.06)" };
  const borderMap = { green: "rgba(34,197,94,0.2)", red: "rgba(239,68,68,0.2)", gray: "rgba(148,163,184,0.2)", indigo: "rgba(99,102,241,0.2)" };
  const textMap = { green: "#16a34a", red: "#dc2626", gray: "#64748b", indigo: "#6366f1" };

  return (
    <div style={{ marginBottom: 12, borderRadius: 10, border: `1px solid ${borderMap[badgeColor]}`, background: bgMap[badgeColor], overflow: "hidden" }}>
      <button
        onClick={() => setOpen(!open)}
        style={{
          width: "100%", padding: "14px 18px", border: "none", background: "transparent", cursor: "pointer",
          display: "flex", alignItems: "center", justifyContent: "space-between", fontFamily: "'DM Sans', sans-serif",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <span style={{
            fontSize: 11, fontWeight: 700, letterSpacing: 0.5, textTransform: "uppercase",
            background: textMap[badgeColor], color: "#fff", padding: "3px 8px", borderRadius: 4,
          }}>{badge}</span>
          <span style={{ fontSize: 15, fontWeight: 600, color: "#1e293b" }}>{title}</span>
          <span style={{ fontSize: 13, color: "#64748b" }}>({count})</span>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 14 }}>
          {amount != null && (
            <span style={{ fontSize: 14, fontWeight: 600, color: textMap[badgeColor] }}>
              {amountLabel} {fmt(amount)}
            </span>
          )}
          <span style={{ fontSize: 18, color: "#94a3b8", transform: open ? "rotate(180deg)" : "rotate(0deg)", transition: "transform 0.2s" }}>▾</span>
        </div>
      </button>
      {open && <div style={{ padding: "0 18px 16px" }}>{children}</div>}
    </div>
  );
}

function StockRow({ stock, type, onClick }) {
  if (type === "tooEarly") {
    const gl = (stock.gain_loss != null) ? stock.gain_loss : (stock.current - stock.invested);
    const glPositive = gl >= 0;
    return (
      <div style={{ display: "grid", gridTemplateColumns: "1.6fr 1fr 1fr 1fr", alignItems: "center", padding: "10px 0", borderBottom: "1px solid rgba(148,163,184,0.15)", gap: 8 }}>
        <div>
          <span style={{ fontSize: 14, fontWeight: 500, color: "#475569" }}>{stock.name}</span>
          <div style={{ fontSize: 11, color: "#94a3b8", fontStyle: "italic" }}>Awaiting 12 months</div>
        </div>
        <div style={{ textAlign: "right" }}>
          <div style={{ fontSize: 13, color: "#475569" }}>{fmt(stock.invested)}</div>
          <div style={{ fontSize: 11, color: "#94a3b8" }}>invested</div>
        </div>
        <div style={{ textAlign: "right" }}>
          <div style={{ fontSize: 13, color: "#475569" }}>{fmt(stock.current)}</div>
          <div style={{ fontSize: 11, color: "#94a3b8" }}>current</div>
        </div>
        <div style={{ textAlign: "right" }}>
          <span style={{ fontSize: 13, fontWeight: 600, color: glPositive ? "#16a34a" : "#dc2626" }}>
            {glPositive ? "+" : "−"}{fmt(Math.abs(gl))}
          </span>
          <div style={{ fontSize: 11, color: "#94a3b8" }}>G/L</div>
        </div>
      </div>
    );
  }

  if (type === "noTest") {
    return (
      <div style={{ display: "grid", gridTemplateColumns: "1.6fr 1fr 1fr", alignItems: "center", padding: "10px 0", borderBottom: "1px solid rgba(148,163,184,0.15)", gap: 8 }}>
        <span style={{ fontSize: 14, fontWeight: 500, color: "#475569" }}>{stock.name}</span>
        <div style={{ textAlign: "right" }}>
          <div style={{ fontSize: 13, color: "#475569" }}>{fmt(stock.invested)}</div>
          <div style={{ fontSize: 11, color: "#94a3b8" }}>invested</div>
        </div>
        <div style={{ textAlign: "right" }}>
          <div style={{ fontSize: 13, color: "#475569" }}>{fmt(stock.current)}</div>
          <div style={{ fontSize: 11, color: "#94a3b8" }}>current</div>
        </div>
      </div>
    );
  }

  const isPass = type === "pass";
  const diffColor = isPass ? "#16a34a" : "#dc2626";
  const diff = isPass ? stock.surplus : stock.deficit;
  const clickable = stock.deep_dive && type === "fail";

  return (
    <div
      onClick={clickable ? onClick : undefined}
      style={{
        display: "grid", gridTemplateColumns: "1.6fr 0.7fr 0.9fr 0.9fr 1fr",
        alignItems: "center", padding: "10px 0",
        borderBottom: "1px solid rgba(148,163,184,0.12)", gap: 8,
        cursor: clickable ? "pointer" : "default",
        borderRadius: clickable ? 6 : 0,
        transition: "background 0.15s",
      }}
      onMouseEnter={(e) => { if (clickable) e.currentTarget.style.background = "rgba(99,102,241,0.06)"; }}
      onMouseLeave={(e) => { if (clickable) e.currentTarget.style.background = "transparent"; }}
    >
      <div>
        <span style={{ fontSize: 14, fontWeight: 500, color: "#1e293b" }}>{stock.name}</span>
        {clickable && <div style={{ fontSize: 11, color: "#6366f1", marginTop: 1 }}>View deep dive →</div>}
        {stock.hurdlePct != null && stock.hurdlePct !== 3 && (
          <div style={{ fontSize: 10, color: "#94a3b8", marginTop: 1 }}>{stock.hurdlePct}% hurdle</div>
        )}
      </div>
      <div style={{ textAlign: "right" }}>
        <span style={{ fontSize: 14, fontWeight: 600, color: isPass ? "#16a34a" : "#dc2626" }}>{stock.cagr}%</span>
        <div style={{ fontSize: 11, color: "#94a3b8" }}>vs {stock.hurdle}%</div>
      </div>
      <div style={{ textAlign: "right" }}>
        <div style={{ fontSize: 13, color: "#475569" }}>{fmt(stock.current)}</div>
        <div style={{ fontSize: 11, color: "#94a3b8" }}>yours</div>
      </div>
      <div style={{ textAlign: "right" }}>
        <div style={{ fontSize: 13, color: "#475569" }}>{fmt(stock.nifty)}</div>
        <div style={{ fontSize: 11, color: "#94a3b8" }}>if Nifty</div>
      </div>
      <div style={{ textAlign: "right" }}>
        <span style={{ fontSize: 14, fontWeight: 700, color: diffColor }}>
          {isPass ? "+" : "−"}{fmt(diff)}
        </span>
      </div>
    </div>
  );
}

function ColHeaders() {
  return (
    <div style={{ display: "grid", gridTemplateColumns: "1.6fr 0.7fr 0.9fr 0.9fr 1fr", padding: "0 0 6px", borderBottom: "1px solid rgba(148,163,184,0.2)", gap: 8 }}>
      {["Stock", "CAGR", "Value", "If Nifty", "Surplus / Deficit"].map((h, i) => (
        <span key={h} style={{ fontSize: 11, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.3, textTransform: "uppercase", textAlign: i === 0 ? "left" : "right" }}>{h}</span>
      ))}
    </div>
  );
}

function LearnMore() {
  const [open, setOpen] = useState(false);
  return (
    <div style={{ marginBottom: 20 }}>
      <button onClick={() => setOpen(!open)} style={{
        background: "none", border: "none", cursor: "pointer", fontSize: 13, color: "#6366f1",
        fontWeight: 500, fontFamily: "'DM Sans', sans-serif", padding: 0, display: "flex", alignItems: "center", gap: 4,
      }}>
        {open ? "▾" : "▸"} Learn more
      </button>
      {open && (
        <div style={{ marginTop: 10, padding: "14px 16px", background: "rgba(99,102,241,0.04)", borderRadius: 8, border: "1px solid rgba(99,102,241,0.12)", fontSize: 13, lineHeight: 1.6, color: "#475569" }}>
          <p style={{ margin: "0 0 8px" }}>
            <strong>Why 3%?</strong> Holding a single stock instead of a diversified index means you carry concentration risk — company-specific events, sector downturns, management missteps. The 3% premium is the minimum compensation you should earn for taking that extra risk.
          </p>
          <p style={{ margin: "0 0 8px" }}>
            <strong>How it works:</strong> For every purchase you made, we calculate what would have happened if that same amount went into the Nifty 500 (total return, including dividends) on the same date. We then compare your stock's CAGR to the Nifty 500 CAGR + hurdle over the same holding period.
          </p>
          <p style={{ margin: 0 }}>
            <strong>Only LTCG lots count.</strong> Purchases held less than 12 months are too early to judge — they show up under "Too Early" with no rating.
          </p>
        </div>
      )}
    </div>
  );
}

export default function TestResults({ data, onBack, onStockClick }) {
  const { stocks, summary } = data;
  const totalTested = summary.pass_count + summary.fail_count + summary.too_early_count + summary.no_test_count;
  const net = summary.total_surplus - summary.total_deficit;
  const isNetPositive = net >= 0;

  return (
    <div>
      {/* Back button */}
      <button
        onClick={onBack}
        style={{
          background: "none", border: "none", cursor: "pointer", fontSize: 13,
          color: "#6366f1", fontWeight: 500, fontFamily: "'DM Sans', sans-serif",
          padding: "0 0 16px", display: "flex", alignItems: "center", gap: 4,
        }}
      >
        ← Back to portfolio
      </button>

      {/* Header */}
      <div style={{ marginBottom: 24 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 4 }}>
          <h1 style={{ fontSize: 22, fontWeight: 700, color: "#0f172a", margin: 0 }}>Hurdle Rate Test</h1>
          <span style={{ fontSize: 11, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.5 }}>FEB 23, 2026</span>
        </div>
        <p style={{ fontSize: 14, color: "#64748b", margin: "4px 0 0", lineHeight: 1.5 }}>
          Is each holding earning enough to justify its risk?
        </p>
        <LearnMore />
      </div>

      {/* Summary strip */}
      <div style={{
        display: "flex", gap: 1, marginBottom: 24, borderRadius: 10, overflow: "hidden",
        border: "1px solid #e2e8f0",
      }}>
        <div style={{ flex: 1, background: "#fff", padding: "16px 18px", textAlign: "center" }}>
          <div style={{ fontSize: 12, color: "#94a3b8", fontWeight: 500, marginBottom: 4 }}>Portfolio</div>
          <div style={{ fontSize: 18, fontWeight: 700, color: "#0f172a" }}>{totalTested} holdings</div>
        </div>
        <div style={{ flex: 1, background: "#fff", padding: "16px 18px", textAlign: "center", borderLeft: "1px solid #f1f5f9", borderRight: "1px solid #f1f5f9" }}>
          <div style={{ fontSize: 12, color: "#16a34a", fontWeight: 600, marginBottom: 4 }}>Passed</div>
          <div style={{ fontSize: 18, fontWeight: 700, color: "#16a34a" }}>{summary.pass_count}</div>
          <div style={{ fontSize: 12, color: "#16a34a", fontWeight: 500 }}>+{fmt(summary.total_surplus)}</div>
        </div>
        <div style={{ flex: 1, background: "#fff", padding: "16px 18px", textAlign: "center", borderRight: "1px solid #f1f5f9" }}>
          <div style={{ fontSize: 12, color: "#dc2626", fontWeight: 600, marginBottom: 4 }}>Failed</div>
          <div style={{ fontSize: 18, fontWeight: 700, color: "#dc2626" }}>{summary.fail_count}</div>
          <div style={{ fontSize: 12, color: "#dc2626", fontWeight: 500 }}>−{fmt(summary.total_deficit)}</div>
        </div>
        <div style={{ flex: 1, background: "#fff", padding: "16px 18px", textAlign: "center" }}>
          <div style={{ fontSize: 12, color: "#64748b", fontWeight: 500, marginBottom: 4 }}>Too Early</div>
          <div style={{ fontSize: 18, fontWeight: 700, color: "#64748b" }}>{summary.too_early_count}</div>
        </div>
      </div>

      {/* Net position */}
      <div style={{
        marginBottom: 24, padding: "14px 18px", borderRadius: 10,
        background: isNetPositive ? "rgba(34,197,94,0.06)" : "rgba(239,68,68,0.06)",
        border: `1px solid ${isNetPositive ? "rgba(34,197,94,0.2)" : "rgba(239,68,68,0.2)"}`,
        display: "flex", justifyContent: "space-between", alignItems: "center",
      }}>
        <span style={{ fontSize: 14, fontWeight: 500, color: "#475569" }}>Net vs benchmark (LTCG lots only)</span>
        <span style={{ fontSize: 17, fontWeight: 700, color: isNetPositive ? "#16a34a" : "#dc2626", fontFamily: "'JetBrains Mono', monospace" }}>
          {isNetPositive ? "+" : "−"}{fmt(Math.abs(net))}
        </span>
      </div>

      {/* Sections */}
      {stocks.fail.length > 0 && (
        <Section title="Failed" badge="FAIL" badgeColor="red" count={stocks.fail.length} amount={summary.total_deficit} amountLabel="Deficit" defaultOpen>
          <ColHeaders />
          {stocks.fail.map((s) => (
            <StockRow key={s.symbol} stock={s} type="fail" onClick={() => onStockClick(s)} />
          ))}
        </Section>
      )}

      {stocks.pass.length > 0 && (
        <Section title="Passed" badge="PASS" badgeColor="green" count={stocks.pass.length} amount={summary.total_surplus} amountLabel="Surplus">
          <ColHeaders />
          {stocks.pass.map((s) => (
            <StockRow key={s.symbol} stock={s} type="pass" />
          ))}
        </Section>
      )}

      {stocks.tooEarly.length > 0 && (
        <Section title="Too Early" badge="WAIT" badgeColor="gray" count={stocks.tooEarly.length}>
          {stocks.tooEarly.map((s) => (
            <StockRow key={s.symbol} stock={s} type="tooEarly" />
          ))}
        </Section>
      )}

      {stocks.noTest.length > 0 && (
        <Section title="No Test" badge="INDEX" badgeColor="indigo" count={stocks.noTest.length}>
          {stocks.noTest.map((s) => (
            <StockRow key={s.symbol} stock={s} type="noTest" />
          ))}
        </Section>
      )}
    </div>
  );
}
