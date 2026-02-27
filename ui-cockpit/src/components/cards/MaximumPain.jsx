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

  const cushioned = phase.stockDD < phase.niftyDD;
  const diff = phase.niftyDD - phase.stockDD;
  const bg = cushioned ? "rgba(34,197,94,0.06)" : "rgba(239,68,68,0.05)";
  const accentColor = cushioned ? "#16a34a" : "#dc2626";
  const barWidth = Math.min(Math.abs(diff) * 3, 100);

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
        <div style={{ fontSize: 15, fontWeight: 600, color: "#1e293b" }}>
          {phase.stockDD.toFixed(1)}%
        </div>
        <div style={{ fontSize: 11, color: "#94a3b8" }}>stock</div>
      </div>
      <div style={{ textAlign: "right" }}>
        <div style={{ fontSize: 15, fontWeight: 500, color: "#64748b" }}>
          {phase.niftyDD.toFixed(1)}%
        </div>
        <div style={{ fontSize: 11, color: "#94a3b8" }}>nifty</div>
      </div>
      <div style={{ paddingLeft: 12 }}>
        <div style={{ fontSize: 12, fontWeight: 700, color: accentColor, marginBottom: 3 }}>
          {cushioned ? `${diff.toFixed(1)}% less` : `${Math.abs(diff).toFixed(1)}% more`}
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

export default function MaximumPain({ stockName, phases }) {
  const cushioned = phases.filter(p => p.listed && p.stockDD < p.niftyDD).length;
  const amplified = phases.filter(p => p.listed && p.stockDD >= p.niftyDD).length;
  const totalPhases = phases.filter(p => p.listed).length;

  return (
    <div style={{ background: "#fff", borderRadius: 12, border: "1px solid #e2e8f0", overflow: "hidden" }}>
      {/* Header */}
      <div style={{ padding: "20px 20px 16px" }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 4 }}>
          <div>
            <span style={{ fontSize: 11, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.5, textTransform: "uppercase" }}>Maximum Pain</span>
            <h2 style={{ fontSize: 18, fontWeight: 700, color: "#0f172a", margin: "4px 0 0" }}>{stockName}</h2>
          </div>
          <div style={{
            display: "flex", alignItems: "center", gap: 6,
            padding: "6px 12px", borderRadius: 8,
            background: cushioned > amplified ? "rgba(34,197,94,0.08)" : "rgba(239,68,68,0.08)",
          }}>
            <span style={{ fontSize: 14, fontWeight: 700, color: "#16a34a" }}>{cushioned}</span>
            <span style={{ fontSize: 12, color: "#94a3b8" }}>/</span>
            <span style={{ fontSize: 14, fontWeight: 700, color: "#dc2626" }}>{amplified}</span>
            <span style={{ fontSize: 12, color: "#94a3b8" }}>phases</span>
          </div>
        </div>
        <p style={{ fontSize: 13, color: "#64748b", margin: "8px 0 0", lineHeight: 1.5 }}>
          Worst peak-to-trough drop in each phase. Green = less pain than market, Red = more pain.
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
        <span style={{ fontSize: 11, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.3, textAlign: "right", paddingLeft: 12 }}>PAIN GAP</span>
      </div>

      {/* Phase rows */}
      {phases.map((p, i) => <PhaseRow key={i} phase={p} />)}

      {/* Summary footer */}
      <div style={{
        padding: "14px 20px", background: "rgba(148,163,184,0.04)",
        display: "flex", justifyContent: "center", alignItems: "center",
      }}>
        <span style={{ fontSize: 13, color: "#64748b" }}>
          Less pain than market in <strong style={{ color: "#16a34a" }}>{cushioned}</strong> of {totalPhases} phases
        </span>
      </div>
    </div>
  );
}
