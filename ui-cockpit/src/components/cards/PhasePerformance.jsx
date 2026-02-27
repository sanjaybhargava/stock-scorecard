function PhaseRow({ phase }) {
  if (!phase.listed) {
    return (
      <div style={{
        display: "grid", gridTemplateColumns: "44px 1fr 100px 100px 90px",
        alignItems: "center", padding: "14px 16px",
        borderBottom: "1px solid rgba(148,163,184,0.1)",
        background: "rgba(148,163,184,0.03)",
      }}>
        <span style={{ fontSize: 20, textAlign: "center", opacity: 0.3 }}>{phase.icon}</span>
        <div>
          <span style={{ fontSize: 14, fontWeight: 500, color: "#94a3b8" }}>{phase.regime}</span>
          <div style={{ fontSize: 12, color: "#cbd5e1" }}>{phase.period}</div>
        </div>
        <div style={{ textAlign: "right", gridColumn: "span 3" }}>
          <span style={{ fontSize: 12, color: "#94a3b8", fontStyle: "italic" }}>Not listed</span>
        </div>
      </div>
    );
  }

  const beat = phase.stockCagr > phase.niftyCagr;
  const diff = phase.stockCagr - phase.niftyCagr;
  const bg = beat ? "rgba(34,197,94,0.06)" : "rgba(239,68,68,0.05)";
  const accentColor = beat ? "#16a34a" : "#dc2626";
  const barWidth = Math.min(Math.abs(diff) * 2.5, 100);

  return (
    <div style={{
      display: "grid", gridTemplateColumns: "44px 1fr 100px 100px 90px",
      alignItems: "center", padding: "14px 16px",
      borderBottom: "1px solid rgba(148,163,184,0.1)",
      background: bg, gap: 0,
    }}>
      <span style={{ fontSize: 20, textAlign: "center", color: phase.regime === "Bear" ? "#dc2626" : phase.regime === "Bull" ? "#16a34a" : "#94a3b8" }}>
        {phase.icon}
      </span>
      <div>
        <span style={{ fontSize: 14, fontWeight: 600, color: "#1e293b" }}>{phase.regime}</span>
        <div style={{ fontSize: 12, color: "#94a3b8", marginTop: 1 }}>{phase.period}</div>
      </div>
      <div style={{ textAlign: "right" }}>
        <div style={{ fontSize: 15, fontWeight: 600, color: phase.stockCagr >= 0 ? "#1e293b" : "#dc2626" }}>
          {phase.stockCagr >= 0 ? "+" : ""}{phase.stockCagr.toFixed(1)}%
        </div>
        <div style={{ fontSize: 11, color: "#94a3b8" }}>stock</div>
      </div>
      <div style={{ textAlign: "right" }}>
        <div style={{ fontSize: 15, fontWeight: 500, color: "#64748b" }}>
          {phase.niftyCagr >= 0 ? "+" : ""}{phase.niftyCagr.toFixed(1)}%
        </div>
        <div style={{ fontSize: 11, color: "#94a3b8" }}>nifty</div>
      </div>
      <div style={{ paddingLeft: 12 }}>
        <div style={{ fontSize: 12, fontWeight: 700, color: accentColor, marginBottom: 3 }}>
          {beat ? "+" : ""}{diff.toFixed(1)}%
        </div>
        <div style={{ height: 4, background: "rgba(148,163,184,0.12)", borderRadius: 2, overflow: "hidden" }}>
          <div style={{
            height: "100%", width: `${barWidth}%`,
            background: accentColor, borderRadius: 2,
            transition: "width 0.4s ease",
          }} />
        </div>
      </div>
    </div>
  );
}

export default function PhasePerformance({ stockName, phases }) {
  const wins = phases.filter(p => p.listed && p.stockCagr > p.niftyCagr).length;
  const losses = phases.filter(p => p.listed && p.stockCagr <= p.niftyCagr).length;
  const totalPhases = phases.filter(p => p.listed).length;

  return (
    <div style={{ background: "#fff", borderRadius: 12, border: "1px solid #e2e8f0", overflow: "hidden" }}>
      {/* Header */}
      <div style={{ padding: "20px 20px 16px" }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 4 }}>
          <div>
            <span style={{ fontSize: 11, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.5, textTransform: "uppercase" }}>Phase Performance</span>
            <h2 style={{ fontSize: 18, fontWeight: 700, color: "#0f172a", margin: "4px 0 0" }}>{stockName}</h2>
          </div>
          <div style={{
            display: "flex", alignItems: "center", gap: 6,
            padding: "6px 12px", borderRadius: 8,
            background: wins > losses ? "rgba(34,197,94,0.08)" : "rgba(239,68,68,0.08)",
          }}>
            <span style={{ fontSize: 14, fontWeight: 700, color: "#16a34a" }}>{wins}</span>
            <span style={{ fontSize: 12, color: "#94a3b8" }}>/</span>
            <span style={{ fontSize: 14, fontWeight: 700, color: "#dc2626" }}>{losses}</span>
            <span style={{ fontSize: 12, color: "#94a3b8" }}>phases</span>
          </div>
        </div>
        <p style={{ fontSize: 13, color: "#64748b", margin: "8px 0 0", lineHeight: 1.5 }}>
          How did this stock perform across different market regimes? Green = beat the market, Red = lagged.
        </p>
      </div>

      {/* Column labels */}
      <div style={{
        display: "grid", gridTemplateColumns: "44px 1fr 100px 100px 90px",
        padding: "0 16px 8px", borderBottom: "1px solid #e2e8f0",
      }}>
        <span></span>
        <span style={{ fontSize: 11, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.3 }}>REGIME</span>
        <span style={{ fontSize: 11, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.3, textAlign: "right" }}>STOCK</span>
        <span style={{ fontSize: 11, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.3, textAlign: "right" }}>NIFTY</span>
        <span style={{ fontSize: 11, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.3, textAlign: "right", paddingLeft: 12 }}>DIFF</span>
      </div>

      {/* Phase rows */}
      {phases.map((p, i) => <PhaseRow key={i} phase={p} />)}

      {/* Summary footer */}
      <div style={{
        padding: "14px 20px", background: "rgba(148,163,184,0.04)",
        display: "flex", justifyContent: "center", alignItems: "center",
      }}>
        <span style={{ fontSize: 13, color: "#64748b" }}>
          Beat market in <strong style={{ color: "#16a34a" }}>{wins}</strong> of {totalPhases} phases
        </span>
      </div>
    </div>
  );
}
