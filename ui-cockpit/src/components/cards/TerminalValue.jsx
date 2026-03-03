import { useState } from "react";
import { fmt, fmtCr } from "../../utils/fmt.js";

function compound(principal, cagr, years) {
  return principal * Math.pow(1 + cagr / 100, years);
}

const MOMENTUM_CAGR = 20;

export default function TerminalValue({ stock }) {
  const { totalShares, price, costPerShare, taxRate, niftyCagr, hcCagr, years } = stock;
  const exemption = stock.ltcgExemption || 125000;
  const defaultSell = Math.round(totalShares * 0.25 / 100) * 100 || Math.round(totalShares * 0.25);
  const [sharesToSell, setSharesToSell] = useState(defaultSell);
  const [hulCagr, setHulCagr] = useState(12);
  const [passivePct, setPassivePct] = useState(80);
  const [convictionPct, setConvictionPct] = useState(60);

  const sharesRetained = totalShares - sharesToSell;

  // SELL side
  const saleValue = sharesToSell * price;
  const costBasis = sharesToSell * costPerShare;
  const gain = saleValue - costBasis;
  const taxableGain = Math.max(0, gain - exemption);
  const tax = Math.round(taxableGain * taxRate / 100);
  const netProceeds = saleValue - tax;

  // Three-bucket allocation
  const activePct = 100 - passivePct;
  const momentumOfActive = 100 - convictionPct;

  const overallNiftyPct = passivePct;
  const overallConvictionPct = activePct * convictionPct / 100;
  const overallMomentumPct = activePct * momentumOfActive / 100;

  const niftyAlloc = netProceeds * overallNiftyPct / 100;
  const convictionAlloc = netProceeds * overallConvictionPct / 100;
  const momentumAlloc = netProceeds * overallMomentumPct / 100;

  const niftyTerminal = compound(niftyAlloc, niftyCagr, years);
  const convictionTerminal = compound(convictionAlloc, hcCagr, years);
  const momentumTerminal = compound(momentumAlloc, MOMENTUM_CAGR, years);
  const redeployTerminal = niftyTerminal + convictionTerminal + momentumTerminal;

  // HOLD side
  const retainedValue = sharesRetained * price;
  const retainedTerminal = compound(retainedValue, hulCagr, years);

  const holdAllTerminal = compound(totalShares * price, hulCagr, years);
  const sellAndRedeployTerminal = redeployTerminal + retainedTerminal;

  const diff = sellAndRedeployTerminal - holdAllTerminal;
  const diffPositive = diff >= 0;

  // Breakeven CAGR solver
  let breakevenCagr = null;
  for (let c = 0; c <= 40; c += 0.1) {
    const holdAll = compound(totalShares * price, c, years);
    const retT = compound((totalShares - sharesToSell) * price, c, years);
    const sellT = redeployTerminal;
    if (holdAll >= sellT + retT) {
      breakevenCagr = c;
      break;
    }
  }

  // Dynamic label for sell & redeploy box
  const redeployLabel = (() => {
    const parts = [];
    parts.push(`${Math.round(overallNiftyPct)}% Nifty @${niftyCagr}%`);
    if (overallConvictionPct > 0) parts.push(`${Math.round(overallConvictionPct)}% picks @${hcCagr}%`);
    if (overallMomentumPct > 0) parts.push(`${Math.round(overallMomentumPct)}% momentum @${MOMENTUM_CAGR}%`);
    return parts.join(" + ");
  })();

  // Dynamic DEPLOYABLE sub-label
  const splitLabel = `${Math.round(overallNiftyPct)}/${Math.round(overallConvictionPct)}/${Math.round(overallMomentumPct)} split`;

  return (
    <div style={{ background: "#fff", borderRadius: 12, border: "1px solid #e2e8f0", overflow: "hidden" }}>

      {/* Header */}
      <div style={{ padding: "20px 20px 16px" }}>
        <span style={{ fontSize: 11, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.5, textTransform: "uppercase" }}>The Decision</span>
        <h2 style={{ fontSize: 18, fontWeight: 700, color: "#0f172a", margin: "4px 0 0" }}>
          Where will you be in {years} years?
        </h2>
        <p style={{ fontSize: 13, color: "#64748b", margin: "8px 0 0", lineHeight: 1.5 }}>
          Sell and redeploy vs hold {stock.name}. Adjust your belief about {stock.name}'s future growth.
        </p>
      </div>

      {/* Slider 1: Shares to sell */}
      <div style={{ padding: "0 20px 16px", borderBottom: "1px solid #e2e8f0" }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
          <span style={{ fontSize: 13, fontWeight: 500, color: "#475569" }}>Shares to sell</span>
          <span style={{ fontSize: 15, fontWeight: 700, color: "#0f172a", fontFamily: "'JetBrains Mono', monospace" }}>
            {sharesToSell.toLocaleString()} of {totalShares.toLocaleString()}
          </span>
        </div>
        <input type="range" min={0} max={totalShares} step={500} value={sharesToSell}
          onChange={e => setSharesToSell(Number(e.target.value))}
          style={{ width: "100%", accentColor: "#6366f1" }}
        />
        <div style={{ display: "flex", justifyContent: "space-between", fontSize: 11, color: "#94a3b8" }}>
          <span>Hold all</span>
          <span>Sell {fmt(saleValue)} → {fmt(netProceeds)} after tax</span>
        </div>
      </div>

      {/* Slider 2: HUL belief */}
      <div style={{ padding: "16px 20px", borderBottom: "1px solid #e2e8f0" }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
          <span style={{ fontSize: 13, fontWeight: 500, color: "#475569" }}>Your {stock.ticker} CAGR belief</span>
          <span style={{ fontSize: 15, fontWeight: 700, color: hulCagr >= (breakevenCagr || 99) ? "#16a34a" : "#dc2626", fontFamily: "'JetBrains Mono', monospace" }}>
            {hulCagr}%
          </span>
        </div>
        <input type="range" min={0} max={30} step={1} value={hulCagr}
          onChange={e => setHulCagr(Number(e.target.value))}
          style={{ width: "100%", accentColor: hulCagr >= (breakevenCagr || 99) ? "#16a34a" : "#dc2626" }}
        />
        <div style={{ display: "flex", justifyContent: "space-between", fontSize: 11, color: "#94a3b8" }}>
          <span>0%</span><span>15%</span><span>30%</span>
        </div>
      </div>

      {/* Slider 3: Passive vs Active */}
      <div style={{ padding: "16px 20px", borderBottom: "1px solid #e2e8f0" }}>
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

      {/* Slider 4: Conviction vs Momentum (only when active > 0) */}
      {activePct > 0 && (
        <div style={{ padding: "16px 20px", borderBottom: "1px solid #e2e8f0" }}>
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

      {/* Hero: Terminal value comparison */}
      {sharesToSell > 0 && (
        <div style={{ padding: "20px" }}>

          {/* Row 1: How much you're selling */}
          <div style={{
            display: "flex", gap: 1, marginBottom: 12, borderRadius: 8, overflow: "hidden",
            border: "1px solid #e2e8f0",
          }}>
            <div style={{ flex: 1, background: "#fff", padding: "12px 14px", textAlign: "center" }}>
              <div style={{ fontSize: 11, color: "#94a3b8", fontWeight: 600, marginBottom: 4 }}>SELLING</div>
              <div style={{ fontSize: 18, fontWeight: 700, color: "#0f172a", fontFamily: "'JetBrains Mono', monospace" }}>
                {sharesToSell.toLocaleString()}
              </div>
              <div style={{ fontSize: 11, color: "#94a3b8" }}>shares</div>
            </div>
            <div style={{ flex: 1, background: "#fff", padding: "12px 14px", textAlign: "center", borderLeft: "1px solid #f1f5f9" }}>
              <div style={{ fontSize: 11, color: "#94a3b8", fontWeight: 600, marginBottom: 4 }}>SALE VALUE</div>
              <div style={{ fontSize: 18, fontWeight: 700, color: "#0f172a", fontFamily: "'JetBrains Mono', monospace" }}>
                {fmt(saleValue)}
              </div>
              <div style={{ fontSize: 11, color: "#94a3b8" }}>@ ₹{price.toLocaleString()}</div>
            </div>
            <div style={{ flex: 1, background: "rgba(239,68,68,0.03)", padding: "12px 14px", textAlign: "center", borderLeft: "1px solid #f1f5f9" }}>
              <div style={{ fontSize: 11, color: "#dc2626", fontWeight: 600, marginBottom: 4 }}>TAX HIT</div>
              <div style={{ fontSize: 18, fontWeight: 700, color: "#dc2626", fontFamily: "'JetBrains Mono', monospace" }}>
                {fmt(tax)}
              </div>
              <div style={{ fontSize: 11, color: "#94a3b8" }}>{taxRate}% on {fmt(taxableGain)}</div>
            </div>
            <div style={{ flex: 1, background: "rgba(99,102,241,0.03)", padding: "12px 14px", textAlign: "center", borderLeft: "1px solid #f1f5f9" }}>
              <div style={{ fontSize: 11, color: "#6366f1", fontWeight: 600, marginBottom: 4 }}>DEPLOYABLE</div>
              <div style={{ fontSize: 18, fontWeight: 700, color: "#6366f1", fontFamily: "'JetBrains Mono', monospace" }}>
                {fmt(netProceeds)}
              </div>
              <div style={{ fontSize: 11, color: "#94a3b8" }}>{splitLabel}</div>
            </div>
          </div>

          {/* Row 2: 5-year terminal value */}
          <div style={{ fontSize: 12, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.3, textTransform: "uppercase", marginBottom: 12, marginTop: 20 }}>
            In {years} years
          </div>

          <div style={{ display: "flex", gap: 12, marginBottom: 16 }}>
            <div style={{
              flex: 1, padding: "16px", borderRadius: 10,
              background: "rgba(148,163,184,0.05)", border: "1px solid rgba(148,163,184,0.2)",
            }}>
              <div style={{ fontSize: 11, fontWeight: 600, color: "#94a3b8", textTransform: "uppercase", marginBottom: 8 }}>
                If you hold all in {stock.ticker}
              </div>
              <div style={{ fontSize: 22, fontWeight: 700, color: "#475569", fontFamily: "'JetBrains Mono', monospace", marginBottom: 4 }}>
                {fmtCr(holdAllTerminal)}
              </div>
              <div style={{ fontSize: 12, color: "#94a3b8" }}>
                {totalShares.toLocaleString()} shares @ {hulCagr}% CAGR
              </div>
            </div>

            <div style={{
              flex: 1, padding: "16px", borderRadius: 10,
              background: diffPositive ? "rgba(34,197,94,0.05)" : "rgba(239,68,68,0.05)",
              border: `1px solid ${diffPositive ? "rgba(34,197,94,0.2)" : "rgba(239,68,68,0.2)"}`,
            }}>
              <div style={{ fontSize: 11, fontWeight: 600, color: "#94a3b8", textTransform: "uppercase", marginBottom: 8 }}>
                If you sell & redeploy
              </div>
              <div style={{ fontSize: 22, fontWeight: 700, color: diffPositive ? "#16a34a" : "#dc2626", fontFamily: "'JetBrains Mono', monospace", marginBottom: 4 }}>
                {fmtCr(sellAndRedeployTerminal)}
              </div>
              <div style={{ fontSize: 12, color: "#94a3b8" }}>
                {redeployLabel}
                {sharesRetained > 0 && ` + ${sharesRetained.toLocaleString()} ${stock.ticker}`}
              </div>
            </div>
          </div>

          {/* Difference callout */}
          <div style={{
            padding: "14px 16px", borderRadius: 8, marginBottom: 16,
            background: diffPositive ? "rgba(34,197,94,0.06)" : "rgba(239,68,68,0.06)",
            border: `1px solid ${diffPositive ? "rgba(34,197,94,0.15)" : "rgba(239,68,68,0.15)"}`,
            display: "flex", justifyContent: "space-between", alignItems: "center",
          }}>
            <span style={{ fontSize: 14, fontWeight: 500, color: "#475569" }}>
              {diffPositive ? "Redeployment wins by" : `Holding ${stock.ticker} wins by`}
            </span>
            <span style={{
              fontSize: 20, fontWeight: 700,
              color: diffPositive ? "#16a34a" : "#dc2626",
              fontFamily: "'JetBrains Mono', monospace",
            }}>
              {fmtCr(Math.abs(diff))}
            </span>
          </div>

          {/* Breakeven CAGR */}
          {breakevenCagr !== null && (
            <div style={{
              padding: "14px 16px", borderRadius: 8,
              background: "rgba(99,102,241,0.04)",
              border: "1px solid rgba(99,102,241,0.15)",
              textAlign: "center",
            }}>
              <div style={{ fontSize: 12, color: "#64748b", marginBottom: 4 }}>
                {stock.ticker} must deliver at least
              </div>
              <span style={{ fontSize: 26, fontWeight: 700, color: "#6366f1", fontFamily: "'JetBrains Mono', monospace" }}>
                {breakevenCagr.toFixed(1)}% CAGR
              </span>
              <div style={{ fontSize: 12, color: "#64748b", marginTop: 4 }}>
                over {years} years for holding to beat selling & redeploying — even after the tax hit
              </div>
            </div>
          )}
        </div>
      )}

      {/* Hold all state */}
      {sharesToSell === 0 && (
        <div style={{ padding: "40px 20px", textAlign: "center" }}>
          <div style={{ fontSize: 15, fontWeight: 600, color: "#475569" }}>You're holding everything</div>
          <div style={{ fontSize: 13, color: "#94a3b8", marginTop: 4 }}>
            Slide right to model selling some or all of your {totalShares.toLocaleString()} shares
          </div>
        </div>
      )}
    </div>
  );
}
