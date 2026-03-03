import { useState, useRef } from "react";
import { fmt } from "../../utils/fmt.js";

const PICKS = [
  { ticker: "SILVERBEES", name: "Nippon India Silver BeES", type: "ETF", entry: 96, exitGTT: 86, rationale: "Silver outperforming gold YTD; industrial + monetary demand tailwind as rate-cut cycle begins." },
  { ticker: "METALIETF", name: "ICICI Pru Nifty Metal ETF", type: "ETF", entry: 12250, exitGTT: 11025, rationale: "Metals leading risk-on rotation; China stimulus + infrastructure capex driving base metal demand." },
  { ticker: "MAZDOCK", name: "Mazagon Dock Shipbuilders", type: "Stock", entry: 2235, exitGTT: 2012, rationale: "Defence strongest secular theme; order book visibility 3+ years, margin expansion on submarine upgrades." },
];

export default function MomentumPicks({ onBack }) {
  const [sliderValue, setSliderValue] = useState(10000000);
  const [methodOpen, setMethodOpen] = useState(false);
  const methodRef = useRef(null);

  const perPick = sliderValue / 3;

  function scrollToMethodology(e) {
    e.preventDefault();
    setMethodOpen(true);
    setTimeout(() => {
      methodRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
    }, 100);
  }

  return (
    <div>
      {/* Back button */}
      <button
        onClick={onBack}
        style={{
          background: "none", border: "none", cursor: "pointer", fontSize: 13,
          color: "#0d9488", fontWeight: 500, fontFamily: "'DM Sans', sans-serif",
          padding: "0 0 16px", display: "flex", alignItems: "center", gap: 4,
        }}
      >
        &larr; Back to portfolio
      </button>

      {/* Header */}
      <div style={{ marginBottom: 24 }}>
        <h1 style={{ fontSize: 22, fontWeight: 700, color: "#0f172a", margin: "0 0 6px" }}>
          Momentum Picks
        </h1>
        <p style={{ fontSize: 14, color: "#64748b", margin: 0, lineHeight: 1.5 }}>
          March 2026 | Monthly Review
        </p>
      </div>

      {/* Disclaimer */}
      <div style={{
        background: "rgba(13,148,136,0.06)", borderRadius: 10, border: "1px solid rgba(13,148,136,0.15)",
        padding: "14px 18px", marginBottom: 20,
      }}>
        <p style={{ fontSize: 13, color: "#475569", margin: 0, lineHeight: 1.6 }}>
          Picked by AI using{" "}
          <a
            href="#methodology"
            onClick={scrollToMethodology}
            style={{ color: "#0d9488", fontWeight: 600, textDecoration: "underline", cursor: "pointer" }}
          >
            this methodology
          </a>
          . Use as a starting point. Do your own research or use a SEBI registered advisor. Momentum investing is high risk.
        </p>
      </div>

      {/* Slider card */}
      <div style={{
        background: "#fff", borderRadius: 12, border: "1px solid #e2e8f0",
        padding: "20px 24px", marginBottom: 20,
      }}>
        <div style={{ fontSize: 12, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.3, textTransform: "uppercase", marginBottom: 12 }}>
          Allocate to momentum picks
        </div>
        <input
          type="range"
          min={0}
          max={10000000}
          step={100000}
          value={sliderValue}
          onChange={(e) => setSliderValue(Number(e.target.value))}
          style={{ width: "100%", accentColor: "#0d9488", marginBottom: 12 }}
        />
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline" }}>
          <div>
            <span style={{ fontSize: 11, color: "#94a3b8" }}>Total allocated</span>
            <div style={{ fontSize: 20, fontWeight: 700, color: "#0f172a", fontFamily: "'JetBrains Mono', monospace" }}>
              {fmt(sliderValue)}
            </div>
          </div>
          <div style={{ textAlign: "right" }}>
            <span style={{ fontSize: 11, color: "#94a3b8" }}>Per pick (÷3)</span>
            <div style={{ fontSize: 20, fontWeight: 700, color: "#0d9488", fontFamily: "'JetBrains Mono', monospace" }}>
              {fmt(perPick)}
            </div>
          </div>
        </div>
      </div>

      {/* Empty state */}
      {sliderValue === 0 ? (
        <div style={{
          background: "#fff", borderRadius: 12, border: "1px solid #e2e8f0",
          padding: "40px 24px", textAlign: "center", marginBottom: 20,
        }}>
          <div style={{ fontSize: 15, color: "#94a3b8", fontWeight: 500 }}>
            Move the slider to allocate capital to momentum picks.
          </div>
        </div>
      ) : (
        <>
          {/* Picks table */}
          <div style={{
            background: "#fff", borderRadius: 12, border: "1px solid #e2e8f0",
            overflow: "hidden", marginBottom: 20,
          }}>
            <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
              <thead>
                <tr style={{ borderBottom: "1px solid #f1f5f9" }}>
                  {["#", "Pick", "Type", "Entry", "Qty", "Exit GTT", "Amount"].map((h) => (
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
                {PICKS.map((pick, i) => {
                  const qty = Math.floor(perPick / pick.entry);
                  const amount = qty * pick.entry;
                  return (
                    <tr key={pick.ticker} style={{ borderBottom: i < PICKS.length - 1 ? "1px solid #f1f5f9" : "none" }}>
                      <td style={{ padding: "10px 14px", textAlign: "center", color: "#94a3b8" }}>
                        {i + 1}
                      </td>
                      <td style={{ padding: "10px 14px" }}>
                        <div style={{ fontWeight: 600, color: "#0f172a" }}>{pick.ticker}</div>
                        <div style={{ fontSize: 11, color: "#94a3b8" }}>{pick.name}</div>
                        <div style={{ fontSize: 11, color: "#64748b", marginTop: 2, lineHeight: 1.4 }}>{pick.rationale}</div>
                      </td>
                      <td style={{ padding: "10px 14px" }}>
                        <span style={{
                          fontSize: 11, fontWeight: 700, textTransform: "uppercase",
                          borderRadius: 4, padding: "3px 8px", lineHeight: 1,
                          background: "#0d9488", color: "#fff",
                        }}>
                          {pick.type}
                        </span>
                      </td>
                      <td style={{ padding: "10px 14px", fontWeight: 700, color: "#0f172a", fontFamily: "'JetBrains Mono', monospace" }}>
                        {fmt(pick.entry)}
                      </td>
                      <td style={{ padding: "10px 14px", fontWeight: 700, color: "#0f172a", fontFamily: "'JetBrains Mono', monospace" }}>
                        {qty.toLocaleString("en-IN")}
                      </td>
                      <td style={{ padding: "10px 14px", fontWeight: 600, color: "#dc2626", fontFamily: "'JetBrains Mono', monospace" }}>
                        {fmt(pick.exitGTT)}
                      </td>
                      <td style={{ padding: "10px 14px", fontWeight: 600, color: "#0f172a", fontFamily: "'JetBrains Mono', monospace" }}>
                        {fmt(amount)}
                      </td>
                    </tr>
                  );
                })}
                {/* Summary row */}
                <tr style={{ background: "rgba(13,148,136,0.04)", borderTop: "1px solid #e2e8f0" }}>
                  <td colSpan={6} style={{ padding: "10px 14px", fontWeight: 600, color: "#0d9488", textAlign: "right" }}>
                    Total deployed
                  </td>
                  <td style={{ padding: "10px 14px", fontWeight: 700, color: "#0d9488", fontFamily: "'JetBrains Mono', monospace" }}>
                    {fmt(PICKS.reduce((sum, p) => sum + Math.floor(perPick / p.entry) * p.entry, 0))}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </>
      )}

      {/* Methodology section */}
      <div
        ref={methodRef}
        id="methodology"
        style={{
          background: "#fff", borderRadius: 12, border: "1px solid #e2e8f0",
          overflow: "hidden",
        }}
      >
        <button
          onClick={() => setMethodOpen(!methodOpen)}
          style={{
            width: "100%", padding: "14px 20px", background: "none", border: "none",
            cursor: "pointer", display: "flex", justifyContent: "space-between", alignItems: "center",
            fontFamily: "'DM Sans', sans-serif",
          }}
        >
          <span style={{ fontSize: 13, fontWeight: 600, color: "#0f172a" }}>
            Learn more: How we picked these
          </span>
          <span style={{ fontSize: 16, color: "#94a3b8", transform: methodOpen ? "rotate(180deg)" : "rotate(0)", transition: "transform 0.2s" }}>
            ▾
          </span>
        </button>
        {methodOpen && (
          <div style={{ padding: "0 20px 18px", fontSize: 13, color: "#475569", lineHeight: 1.7 }}>
            <p style={{ margin: "0 0 12px" }}>
              We use a rules-based momentum screen that runs on the last trading day of each month. The universe is NSE-listed stocks and ETFs with average daily turnover above ₹5 Cr. We rank by 6-month price return, exclude the top 5% (blow-off risk) and anything below its 200-day moving average, then pick the top 3 names with the best risk-adjusted trend score (return ÷ max drawdown over the lookback).
            </p>
            <p style={{ margin: 0 }}>
              Exit discipline is mechanical: a 10% trailing stop from entry price is placed as a GTT sell order on day one. If a pick is stopped out mid-month it is not replaced until the next monthly review. The goal is to ride strong trends while cutting losers early — no discretion, no hope-and-hold.
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
