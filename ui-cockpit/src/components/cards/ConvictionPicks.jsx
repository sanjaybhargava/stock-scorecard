import { useState } from "react";
import { fmt } from "../../utils/fmt.js";

const MAX_REDEPLOYABLE = 74318246;

const PICKS = [
  { id: 1, ticker: "KFINTECH", name: "KFin Tech", ceo: "Sreekanth Nadella", status: "active", strategy: "Core", role: "Financial Infrastructure / SaaS", resultDate: "Mid May 2026" },
  { id: 2, ticker: "IDFCFIRSTB", name: "IDFC First Bank", ceo: "V. Vaidyanathan", status: "active", strategy: "Opportunistic", role: "Banking Value (Recovery Play)", resultDate: "Late April 2026" },
  { id: 3, ticker: "AMARAJABAT", name: "Amara Raja", ceo: "Jayadev Galla", status: "active", strategy: "Core", role: "Energy Transition / Gigafactory", resultDate: "Late May 2026" },
  { id: 4, ticker: "SYRMA", name: "Syrma SGS", ceo: "J.S. Gujral", status: "active", strategy: "Core", role: "Defense & Industrial Electronics", resultDate: "Mid May 2026" },
  { id: 5, ticker: "SOLARINDS", name: "Solar Industries", ceo: "Manish Nuwal", status: "gtt", gttPrice: 11800, strategy: "Structural", role: "Ammunition & Rocket Propellants", resultDate: "Late May 2026" },
  { id: 6, ticker: "ZENTEC", name: "Zen Technologies", ceo: "Ashok Atluri", status: "gtt", gttPrice: 1180, strategy: "Structural", role: "Drone Warfare & Anti-Drone Tech", resultDate: "Early May 2026" },
  { id: 7, ticker: "PTCINDIA", name: "PTC Industries", ceo: "Sachin Agarwal", status: "gtt", gttPrice: 17200, strategy: "Moat", role: "Strategic Titanium Castings", resultDate: "Late May 2026" },
  { id: 8, ticker: "NETWEB", name: "Netweb Technologies", ceo: "Sanjay Lodha", status: "gtt", gttPrice: 3000, strategy: "Tech", role: "AI Infrastructure & Servers", resultDate: "Early May 2026" },
  { id: 9, ticker: "GRAVITA", name: "Gravita India", ceo: "Yogesh Malhotra", status: "gtt", gttPrice: 1500, strategy: "Resilience", role: "Circular Economy (Metal Recycling)", resultDate: "Early May 2026" },
  { id: 10, ticker: "APARINDS", name: "Apar Industries", ceo: "Kushal Desai", status: "gtt", gttPrice: 9200, strategy: "Infra", role: "Global Power Transmission", resultDate: "Late May 2026" },
  { id: 11, ticker: "KEI", name: "KEI Industries", ceo: "Anil Gupta", status: "gtt", gttPrice: 4180, strategy: "Infra", role: "Urban Underground Cables", resultDate: "Mid May 2026" },
];

const activePicks = PICKS.filter((p) => p.status === "active");
const gttPicks = PICKS.filter((p) => p.status === "gtt");

export default function ConvictionPicks({ onBack }) {
  const [sliderValue, setSliderValue] = useState(MAX_REDEPLOYABLE);

  const convictionSlice = sliderValue * 0.12;
  const perStock = convictionSlice / 11;
  const activeTotal = perStock * activePicks.length;
  const gttTotal = perStock * gttPicks.length;

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
        &larr; Back to portfolio
      </button>

      {/* Header */}
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ fontSize: 22, fontWeight: 700, color: "#0f172a", margin: "0 0 6px" }}>
          High-Conviction Picks
        </h1>
        <p style={{ fontSize: 14, color: "#64748b", margin: 0, lineHeight: 1.5 }}>
          11 stocks targeting 25% CAGR &mdash; 3.05&times; in 5 years. 4 active at market, 7 waiting at GTT prices.
        </p>
      </div>

      {/* Slider card */}
      <div style={{
        background: "#fff", borderRadius: 12, border: "1px solid #e2e8f0",
        padding: "20px 24px", marginBottom: 20,
      }}>
        <div style={{ fontSize: 12, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.3, textTransform: "uppercase", marginBottom: 12 }}>
          Redeployable Capital
        </div>
        <input
          type="range"
          min={0}
          max={MAX_REDEPLOYABLE}
          step={500000}
          value={sliderValue}
          onChange={(e) => setSliderValue(Number(e.target.value))}
          style={{ width: "100%", accentColor: "#6366f1", marginBottom: 12 }}
        />
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline" }}>
          <div>
            <span style={{ fontSize: 11, color: "#94a3b8" }}>Total redeployable</span>
            <div style={{ fontSize: 20, fontWeight: 700, color: "#0f172a", fontFamily: "'JetBrains Mono', monospace" }}>
              {fmt(sliderValue)}
            </div>
          </div>
          <div style={{ textAlign: "right" }}>
            <span style={{ fontSize: 11, color: "#94a3b8" }}>Conviction slice (12%)</span>
            <div style={{ fontSize: 20, fontWeight: 700, color: "#d97706", fontFamily: "'JetBrains Mono', monospace" }}>
              {fmt(convictionSlice)}
            </div>
          </div>
        </div>
      </div>

      {/* Empty state */}
      {sliderValue === 0 ? (
        <div style={{
          background: "#fff", borderRadius: 12, border: "1px solid #e2e8f0",
          padding: "40px 24px", textAlign: "center",
        }}>
          <div style={{ fontSize: 15, color: "#94a3b8", fontWeight: 500 }}>
            Move the slider to allocate capital to conviction picks
          </div>
        </div>
      ) : (
        <>
          {/* Deployment summary bar */}
          <div style={{
            display: "flex", gap: 1, marginBottom: 20, borderRadius: 8,
            overflow: "hidden", border: "1px solid #e2e8f0",
          }}>
            <div style={{
              flex: activePicks.length, background: "rgba(99,102,241,0.06)",
              padding: "12px 14px", textAlign: "center",
            }}>
              <div style={{ fontSize: 12, color: "#6366f1", fontWeight: 600, marginBottom: 2 }}>
                {activePicks.length} Active
              </div>
              <div style={{ fontSize: 16, fontWeight: 700, color: "#6366f1", fontFamily: "'JetBrains Mono', monospace" }}>
                {fmt(activeTotal)}
              </div>
              <div style={{ fontSize: 11, color: "#94a3b8" }}>at market price</div>
            </div>
            <div style={{
              flex: gttPicks.length, background: "rgba(217,119,6,0.06)",
              padding: "12px 14px", textAlign: "center",
            }}>
              <div style={{ fontSize: 12, color: "#d97706", fontWeight: 600, marginBottom: 2 }}>
                {gttPicks.length} GTT
              </div>
              <div style={{ fontSize: 16, fontWeight: 700, color: "#d97706", fontFamily: "'JetBrains Mono', monospace" }}>
                {fmt(gttTotal)}
              </div>
              <div style={{ fontSize: 11, color: "#94a3b8" }}>waiting at limit price</div>
            </div>
          </div>

          {/* Pick rows */}
          <div style={{
            background: "#fff", borderRadius: 12, border: "1px solid #e2e8f0",
            overflow: "hidden", marginBottom: 20,
          }}>
            {PICKS.map((pick, i) => (
              <div
                key={pick.id}
                style={{
                  padding: "14px 20px",
                  borderBottom: i < PICKS.length - 1 ? "1px solid #f1f5f9" : "none",
                  display: "flex", alignItems: "center", justifyContent: "space-between",
                }}
              >
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 4 }}>
                    <span style={{ fontSize: 14, fontWeight: 600, color: "#0f172a" }}>
                      {pick.name}
                    </span>
                    <span style={{
                      fontSize: 11, fontWeight: 700, textTransform: "uppercase",
                      borderRadius: 4, padding: "3px 8px", lineHeight: 1,
                      background: pick.status === "active" ? "#6366f1" : "#d97706",
                      color: "#fff",
                    }}>
                      {pick.status === "active" ? "ACTIVE" : `GTT ${fmt(pick.gttPrice)}`}
                    </span>
                  </div>
                  <div style={{ fontSize: 12, color: "#64748b" }}>
                    {pick.ceo} &middot; {pick.role}
                  </div>
                  <div style={{ fontSize: 11, color: "#94a3b8", marginTop: 2 }}>
                    {pick.strategy} &middot; Q4 results {pick.resultDate}
                  </div>
                </div>
                <div style={{ textAlign: "right", flexShrink: 0, marginLeft: 12 }}>
                  <div style={{ fontSize: 14, fontWeight: 700, color: "#0f172a", fontFamily: "'JetBrains Mono', monospace" }}>
                    {fmt(perStock)}
                  </div>
                  {pick.status === "gtt" && (
                    <div style={{ fontSize: 11, color: "#94a3b8" }}>
                      {Math.floor(perStock / pick.gttPrice).toLocaleString("en-IN")} shares
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>

          {/* GTT matrix table */}
          <div style={{
            background: "#fff", borderRadius: 12, border: "1px solid #e2e8f0",
            overflow: "hidden",
          }}>
            <div style={{
              padding: "14px 20px", borderBottom: "1px solid #e2e8f0",
              fontSize: 12, fontWeight: 600, color: "#94a3b8",
              letterSpacing: 0.3, textTransform: "uppercase",
            }}>
              GTT Order Matrix
            </div>
            <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
              <thead>
                <tr style={{ borderBottom: "1px solid #f1f5f9" }}>
                  {["#", "Ticker", "Entry", "Amount", "Shares"].map((h) => (
                    <th
                      key={h}
                      style={{
                        padding: "10px 14px", textAlign: h === "#" ? "center" : "left",
                        fontSize: 11, fontWeight: 600, color: "#94a3b8",
                        textTransform: "uppercase", letterSpacing: 0.3,
                      }}
                    >
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {gttPicks.map((pick) => {
                  const shares = Math.floor(perStock / pick.gttPrice);
                  return (
                    <tr key={pick.id} style={{ borderBottom: "1px solid #f1f5f9" }}>
                      <td style={{ padding: "10px 14px", textAlign: "center", color: "#94a3b8" }}>
                        {pick.id}
                      </td>
                      <td style={{ padding: "10px 14px", fontWeight: 600, color: "#0f172a" }}>
                        {pick.ticker}
                      </td>
                      <td style={{ padding: "10px 14px", fontFamily: "'JetBrains Mono', monospace", color: "#0f172a" }}>
                        {fmt(pick.gttPrice)}
                      </td>
                      <td style={{ padding: "10px 14px", fontFamily: "'JetBrains Mono', monospace", color: "#0f172a" }}>
                        {fmt(perStock)}
                      </td>
                      <td style={{ padding: "10px 14px", fontFamily: "'JetBrains Mono', monospace", color: "#0f172a" }}>
                        {shares.toLocaleString("en-IN")}
                      </td>
                    </tr>
                  );
                })}
                {/* Summary row */}
                <tr style={{ background: "rgba(217,119,6,0.04)" }}>
                  <td colSpan={3} style={{ padding: "10px 14px", fontWeight: 600, color: "#d97706", textAlign: "right" }}>
                    Total GTT
                  </td>
                  <td style={{ padding: "10px 14px", fontWeight: 700, color: "#d97706", fontFamily: "'JetBrains Mono', monospace" }}>
                    {fmt(gttTotal)}
                  </td>
                  <td style={{ padding: "10px 14px", fontWeight: 700, color: "#d97706", fontFamily: "'JetBrains Mono', monospace" }}>
                    {gttPicks.reduce((sum, p) => sum + Math.floor(perStock / p.gttPrice), 0).toLocaleString("en-IN")}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  );
}
