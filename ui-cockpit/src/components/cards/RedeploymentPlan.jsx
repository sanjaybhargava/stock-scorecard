import { useState } from "react";
import { fmt } from "../../utils/fmt.js";

function computePlan(stock, exitPct) {
  const sharesExiting = Math.round(stock.shares * exitPct / 100);
  const marketValue = sharesExiting * stock.price;
  const costBasis = sharesExiting * stock.costPerShare;
  const gain = marketValue - costBasis;
  const exemption = stock.ltcgExemption || 125000;
  const taxableGain = Math.max(0, gain - exemption);
  const tax = Math.round(taxableGain * stock.taxRate / 100);
  const netProceeds = marketValue - tax;
  return { sharesExiting, marketValue, costBasis, gain, taxableGain, exemption, tax, netProceeds };
}

function FlowRow({ label, amount, sub, color, bold, border }) {
  return (
    <div style={{
      display: "flex", justifyContent: "space-between", alignItems: "center",
      padding: "10px 0",
      borderBottom: border ? "1px solid rgba(148,163,184,0.12)" : "none",
    }}>
      <div>
        <span style={{ fontSize: 14, fontWeight: bold ? 600 : 400, color: color || "#475569" }}>{label}</span>
        {sub && <div style={{ fontSize: 12, color: "#94a3b8", marginTop: 1 }}>{sub}</div>}
      </div>
      <span style={{
        fontSize: bold ? 17 : 15, fontWeight: bold ? 700 : 500,
        color: color || "#1e293b",
        fontFamily: "'JetBrains Mono', monospace",
      }}>{amount}</span>
    </div>
  );
}

export default function RedeploymentPlan({ stock }) {
  const [exitPct, setExitPct] = useState(100);
  const [passivePct, setPassivePct] = useState(80);
  const [convictionPct, setConvictionPct] = useState(60);

  const plan = computePlan(stock, exitPct);
  const activePct = 100 - passivePct;
  const momentumOfActive = 100 - convictionPct;

  const overallNiftyPct = passivePct;
  const overallConvictionPct = activePct * convictionPct / 100;
  const overallMomentumPct = activePct * momentumOfActive / 100;

  const niftyAmount = Math.round(plan.netProceeds * overallNiftyPct / 100);
  const convictionAmount = Math.round(plan.netProceeds * overallConvictionPct / 100);
  const momentumAmount = Math.round(plan.netProceeds * overallMomentumPct / 100);
  const sharesRemaining = stock.shares - plan.sharesExiting;

  return (
    <div style={{ background: "#fff", borderRadius: 12, border: "1px solid #e2e8f0", overflow: "hidden" }}>

      {/* Header */}
      <div style={{ padding: "20px 20px 16px" }}>
        <span style={{ fontSize: 11, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.5, textTransform: "uppercase" }}>Redeployment Plan</span>
        <h2 style={{ fontSize: 18, fontWeight: 700, color: "#0f172a", margin: "4px 0 0" }}>
          If you exit {stock.name} {exitPct < 100 ? "partially" : "fully"}
        </h2>
        <p style={{ fontSize: 13, color: "#64748b", margin: "8px 0 0", lineHeight: 1.5 }}>
          Sell → pay tax → redeploy. Adjust the sliders to model different scenarios.
        </p>
      </div>

      {/* Exit slider */}
      <div style={{ padding: "0 20px 16px", borderBottom: "1px solid #e2e8f0" }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
          <span style={{ fontSize: 13, fontWeight: 500, color: "#475569" }}>Exit how much?</span>
          <span style={{ fontSize: 15, fontWeight: 700, color: "#0f172a", fontFamily: "'JetBrains Mono', monospace" }}>
            {exitPct}% — {plan.sharesExiting.toLocaleString()} shares
          </span>
        </div>
        <input type="range" min={10} max={100} step={10} value={exitPct}
          onChange={e => setExitPct(Number(e.target.value))}
          style={{ width: "100%", accentColor: "#6366f1" }}
        />
        <div style={{ display: "flex", justifyContent: "space-between", fontSize: 11, color: "#94a3b8" }}>
          <span>10%</span><span>50%</span><span>100%</span>
        </div>
        {exitPct < 100 && (
          <div style={{ marginTop: 8, padding: "8px 12px", borderRadius: 6, background: "rgba(99,102,241,0.05)", fontSize: 12, color: "#6366f1" }}>
            Retaining {sharesRemaining.toLocaleString()} shares ({fmt(sharesRemaining * stock.price)}) + covered call income on those
          </div>
        )}
      </div>

      {/* Waterfall */}
      <div style={{ padding: "0 20px 16px", borderBottom: "1px solid #e2e8f0" }}>
        <div style={{ fontSize: 12, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.3, textTransform: "uppercase", marginBottom: 8, marginTop: 16 }}>Proceeds after tax</div>
        <FlowRow label="Sale value" amount={fmt(plan.marketValue)} sub={`${plan.sharesExiting.toLocaleString()} shares × ₹${stock.price.toLocaleString()}`} border />
        <FlowRow label="LTCG tax" amount={`−${fmt(plan.tax)}`} sub={`${stock.taxRate}% on ${fmt(plan.taxableGain)} (gain ${fmt(plan.gain)} − ${fmt(plan.exemption)} exempt)`} color="#dc2626" border />
        <FlowRow label="Net deployable" amount={fmt(plan.netProceeds)} bold color="#0f172a" />
      </div>

      {/* Allocation split */}
      <div style={{ padding: "16px 20px", borderBottom: "1px solid #e2e8f0" }}>
        <div style={{ fontSize: 12, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.3, textTransform: "uppercase", marginBottom: 12 }}>
          Your allocation
        </div>

        {/* Slider 1: Passive vs Active */}
        <div style={{ marginBottom: 16 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
            <span style={{ fontSize: 13, fontWeight: 500, color: "#475569" }}>How much goes to the index?</span>
            <span style={{ fontSize: 13, fontWeight: 600, color: "#6366f1", fontFamily: "'JetBrains Mono', monospace" }}>
              Passive {passivePct}% / Active {activePct}%
            </span>
          </div>
          <input type="range" min={50} max={100} step={10} value={passivePct}
            onChange={e => setPassivePct(Number(e.target.value))}
            style={{ width: "100%", accentColor: "#6366f1" }}
          />
        </div>

        {/* Slider 2: Conviction vs Momentum (only when active > 0) */}
        {activePct > 0 && (
          <div style={{ marginBottom: 16 }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
              <span style={{ fontSize: 13, fontWeight: 500, color: "#475569" }}>Within your active bet — patient capital vs tactical?</span>
              <span style={{ fontSize: 13, fontWeight: 600, color: "#d97706", fontFamily: "'JetBrains Mono', monospace" }}>
                Conviction {convictionPct}% / Momentum {momentumOfActive}%
              </span>
            </div>
            <input type="range" min={0} max={100} step={10} value={convictionPct}
              onChange={e => setConvictionPct(Number(e.target.value))}
              style={{ width: "100%", accentColor: "#d97706" }}
            />
          </div>
        )}

        {/* Progress bar: three segments */}
        <div style={{ display: "flex", height: 8, borderRadius: 4, overflow: "hidden", marginBottom: 16 }}>
          <div style={{ width: `${overallNiftyPct}%`, background: "#6366f1", transition: "width 0.2s" }} />
          <div style={{ width: `${overallConvictionPct}%`, background: "#d97706", transition: "width 0.2s" }} />
          <div style={{ width: `${overallMomentumPct}%`, background: "#0d9488", transition: "width 0.2s" }} />
        </div>

        {/* Three allocation boxes */}
        <div style={{ display: "flex", gap: 12 }}>
          {/* Nifty 500 */}
          <div style={{ flex: 1, padding: "14px 16px", borderRadius: 10, background: "rgba(99,102,241,0.04)", border: "1px solid rgba(99,102,241,0.15)" }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
              <span style={{ fontSize: 12, fontWeight: 700, color: "#6366f1" }}>{Math.round(overallNiftyPct)}%</span>
              <span style={{ fontSize: 16, fontWeight: 700, color: "#6366f1", fontFamily: "'JetBrains Mono', monospace" }}>{fmt(niftyAmount)}</span>
            </div>
            <div style={{ fontSize: 14, fontWeight: 600, color: "#1e293b", marginBottom: 4 }}>Nifty 500 Index</div>
            <div style={{ fontSize: 12, color: "#64748b", lineHeight: 1.5 }}>
              Broad market, no concentration risk. 16.2% CAGR over the last 10 years.
            </div>
          </div>

          {/* High-Conviction */}
          <div style={{ flex: 1, padding: "14px 16px", borderRadius: 10, background: "rgba(217,119,6,0.04)", border: "1px solid rgba(217,119,6,0.15)" }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
              <span style={{ fontSize: 12, fontWeight: 700, color: "#d97706" }}>{Math.round(overallConvictionPct)}%</span>
              <span style={{ fontSize: 16, fontWeight: 700, color: "#d97706", fontFamily: "'JetBrains Mono', monospace" }}>{fmt(convictionAmount)}</span>
            </div>
            <div style={{ fontSize: 14, fontWeight: 600, color: "#1e293b", marginBottom: 4 }}>High-Conviction Picks</div>
            <div style={{ fontSize: 12, color: "#64748b", lineHeight: 1.5 }}>
              3–5 year holds, 3% test discipline. 25% CAGR target.
            </div>
          </div>

          {/* Momentum */}
          <div style={{ flex: 1, padding: "14px 16px", borderRadius: 10, background: "rgba(13,148,136,0.04)", border: "1px solid rgba(13,148,136,0.15)" }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
              <span style={{ fontSize: 12, fontWeight: 700, color: "#0d9488" }}>{Math.round(overallMomentumPct)}%</span>
              <span style={{ fontSize: 16, fontWeight: 700, color: "#0d9488", fontFamily: "'JetBrains Mono', monospace" }}>{fmt(momentumAmount)}</span>
            </div>
            <div style={{ fontSize: 14, fontWeight: 600, color: "#1e293b", marginBottom: 4 }}>Momentum</div>
            <div style={{ fontSize: 12, color: "#64748b", lineHeight: 1.5 }}>
              AI-assisted macro themes · 6–18 month holds. 20% CAGR target.
            </div>
          </div>
        </div>
      </div>

      {/* Risk profile */}
      <div style={{ padding: "16px 20px", background: "rgba(148,163,184,0.02)" }}>
        <div style={{ fontSize: 12, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.3, textTransform: "uppercase", marginBottom: 10 }}>
          Risk profile across your allocation
        </div>
        <div style={{ display: "flex", gap: 12 }}>
          {/* Nifty 500 risk */}
          <div style={{ flex: 1, borderRadius: 8, overflow: "hidden", border: "1px solid rgba(99,102,241,0.15)", background: "rgba(99,102,241,0.04)" }}>
            <div style={{ padding: "8px 12px", textAlign: "center", borderBottom: "1px solid rgba(99,102,241,0.1)" }}>
              <div style={{ fontSize: 11, fontWeight: 600, color: "#6366f1", textTransform: "uppercase" }}>Nifty 500</div>
            </div>
            <div style={{ padding: "10px 12px", textAlign: "center", borderBottom: "1px solid rgba(99,102,241,0.08)" }}>
              <div style={{ fontSize: 11, color: "#94a3b8", marginBottom: 2 }}>Downside</div>
              <div style={{ fontSize: 16, fontWeight: 700, color: "#6366f1" }}>10%</div>
              <div style={{ fontSize: 11, color: "#94a3b8" }}>CAGR</div>
            </div>
            <div style={{ padding: "10px 12px", textAlign: "center", borderBottom: "1px solid rgba(99,102,241,0.08)" }}>
              <div style={{ fontSize: 11, color: "#94a3b8", marginBottom: 2 }}>Target</div>
              <div style={{ fontSize: 16, fontWeight: 700, color: "#6366f1" }}>16.2%</div>
              <div style={{ fontSize: 11, color: "#94a3b8" }}>CAGR</div>
            </div>
            <div style={{ padding: "10px 12px", textAlign: "center" }}>
              <div style={{ fontSize: 11, color: "#94a3b8", marginBottom: 2 }}>Upside</div>
              <div style={{ fontSize: 16, fontWeight: 700, color: "#6366f1" }}>22%+</div>
              <div style={{ fontSize: 11, color: "#94a3b8" }}>CAGR</div>
            </div>
          </div>

          {/* High-Conviction risk */}
          <div style={{ flex: 1, borderRadius: 8, overflow: "hidden", border: "1px solid rgba(217,119,6,0.15)", background: "rgba(217,119,6,0.04)" }}>
            <div style={{ padding: "8px 12px", textAlign: "center", borderBottom: "1px solid rgba(217,119,6,0.1)" }}>
              <div style={{ fontSize: 11, fontWeight: 600, color: "#d97706", textTransform: "uppercase" }}>High-Conviction</div>
            </div>
            <div style={{ padding: "10px 12px", textAlign: "center", borderBottom: "1px solid rgba(217,119,6,0.08)" }}>
              <div style={{ fontSize: 11, color: "#94a3b8", marginBottom: 2 }}>Downside</div>
              <div style={{ fontSize: 16, fontWeight: 700, color: "#d97706" }}>5%</div>
              <div style={{ fontSize: 11, color: "#94a3b8" }}>CAGR</div>
            </div>
            <div style={{ padding: "10px 12px", textAlign: "center", borderBottom: "1px solid rgba(217,119,6,0.08)" }}>
              <div style={{ fontSize: 11, color: "#94a3b8", marginBottom: 2 }}>Target</div>
              <div style={{ fontSize: 16, fontWeight: 700, color: "#d97706" }}>25%</div>
              <div style={{ fontSize: 11, color: "#94a3b8" }}>CAGR</div>
            </div>
            <div style={{ padding: "10px 12px", textAlign: "center" }}>
              <div style={{ fontSize: 11, color: "#94a3b8", marginBottom: 2 }}>Upside</div>
              <div style={{ fontSize: 16, fontWeight: 700, color: "#d97706" }}>40%+</div>
              <div style={{ fontSize: 11, color: "#94a3b8" }}>CAGR</div>
            </div>
          </div>

          {/* Momentum risk */}
          <div style={{ flex: 1, borderRadius: 8, overflow: "hidden", border: "1px solid rgba(13,148,136,0.15)", background: "rgba(13,148,136,0.04)" }}>
            <div style={{ padding: "8px 12px", textAlign: "center", borderBottom: "1px solid rgba(13,148,136,0.1)" }}>
              <div style={{ fontSize: 11, fontWeight: 600, color: "#0d9488", textTransform: "uppercase" }}>Momentum</div>
            </div>
            <div style={{ padding: "10px 12px", textAlign: "center", borderBottom: "1px solid rgba(13,148,136,0.08)" }}>
              <div style={{ fontSize: 11, color: "#94a3b8", marginBottom: 2 }}>Downside</div>
              <div style={{ fontSize: 16, fontWeight: 700, color: "#dc2626" }}>−10%</div>
              <div style={{ fontSize: 11, color: "#94a3b8" }}>CAGR</div>
            </div>
            <div style={{ padding: "10px 12px", textAlign: "center", borderBottom: "1px solid rgba(13,148,136,0.08)" }}>
              <div style={{ fontSize: 11, color: "#94a3b8", marginBottom: 2 }}>Target</div>
              <div style={{ fontSize: 16, fontWeight: 700, color: "#0d9488" }}>20%</div>
              <div style={{ fontSize: 11, color: "#94a3b8" }}>CAGR</div>
            </div>
            <div style={{ padding: "10px 12px", textAlign: "center" }}>
              <div style={{ fontSize: 11, color: "#94a3b8", marginBottom: 2 }}>Upside</div>
              <div style={{ fontSize: 16, fontWeight: 700, color: "#0d9488" }}>35%+</div>
              <div style={{ fontSize: 11, color: "#94a3b8" }}>CAGR</div>
            </div>
          </div>
        </div>
        <p style={{ fontSize: 12, color: "#64748b", margin: "10px 0 0", lineHeight: 1.5, textAlign: "center" }}>
          Your {Math.round(overallNiftyPct)}% index core delivers steady compounding. The {Math.round(overallConvictionPct)}% conviction and {Math.round(overallMomentumPct)}% momentum bets add return potential with managed risk.
        </p>
      </div>
    </div>
  );
}
