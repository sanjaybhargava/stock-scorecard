import { useState } from "react";

function CasePoint({ point, color }) {
  const [open, setOpen] = useState(false);
  const tagBg = color === "green" ? "rgba(34,197,94,0.1)" : "rgba(239,68,68,0.1)";
  const tagColor = color === "green" ? "#16a34a" : "#dc2626";

  return (
    <div style={{ marginBottom: 2 }}>
      <button
        onClick={() => setOpen(!open)}
        style={{
          width: "100%", padding: "10px 0", border: "none", background: "transparent",
          cursor: "pointer", display: "flex", alignItems: "center", gap: 10,
          fontFamily: "'DM Sans', sans-serif", textAlign: "left",
          borderBottom: "1px solid rgba(148,163,184,0.08)",
        }}
      >
        <span style={{
          fontSize: 10, fontWeight: 700, letterSpacing: 0.5, textTransform: "uppercase",
          background: tagBg, color: tagColor, padding: "3px 7px", borderRadius: 4,
          flexShrink: 0, minWidth: 70, textAlign: "center",
        }}>{point.tag}</span>
        <span style={{ fontSize: 13, fontWeight: 500, color: "#1e293b", flex: 1 }}>{point.title}</span>
        <span style={{
          fontSize: 14, color: "#94a3b8", transform: open ? "rotate(180deg)" : "rotate(0deg)",
          transition: "transform 0.2s", flexShrink: 0,
        }}>▾</span>
      </button>
      {open && (
        <div style={{
          padding: "8px 0 12px 82px", fontSize: 12, color: "#64748b", lineHeight: 1.6,
          borderBottom: "1px solid rgba(148,163,184,0.08)",
        }}>
          {point.detail}
        </div>
      )}
    </div>
  );
}

export default function FutureProspects({ stockName, bullPoints, bearPoints, verdict }) {
  const [showBull, setShowBull] = useState(true);
  const [showBear, setShowBear] = useState(true);

  return (
    <div style={{ background: "#fff", borderRadius: 12, border: "1px solid #e2e8f0", overflow: "hidden" }}>

      {/* Header */}
      <div style={{ padding: "20px 20px 16px" }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 4 }}>
          <div>
            <span style={{ fontSize: 11, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.5, textTransform: "uppercase" }}>Future Prospects</span>
            <h2 style={{ fontSize: 18, fontWeight: 700, color: "#0f172a", margin: "4px 0 0" }}>{stockName}</h2>
          </div>
          <div style={{ fontSize: 12, color: "#64748b" }}>As of Feb 2026</div>
        </div>
        <p style={{ fontSize: 13, color: "#64748b", margin: "8px 0 0", lineHeight: 1.5 }}>
          The stock failed the 3% test over the last 10 years. Can the next 5 be different?
        </p>
      </div>

      {/* Bull case */}
      <div style={{ borderTop: "1px solid #e2e8f0" }}>
        <button
          onClick={() => setShowBull(!showBull)}
          style={{
            width: "100%", padding: "14px 20px", border: "none", background: "rgba(34,197,94,0.04)",
            cursor: "pointer", display: "flex", alignItems: "center", justifyContent: "space-between",
            fontFamily: "'DM Sans', sans-serif",
          }}
        >
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <span style={{ fontSize: 14, fontWeight: 600, color: "#16a34a" }}>The Bull Case</span>
            <span style={{ fontSize: 12, color: "#94a3b8" }}>({bullPoints.length} factors)</span>
          </div>
          <span style={{
            fontSize: 18, color: "#94a3b8",
            transform: showBull ? "rotate(180deg)" : "rotate(0deg)", transition: "transform 0.2s",
          }}>▾</span>
        </button>
        {showBull && (
          <div style={{ padding: "0 20px 8px" }}>
            {bullPoints.map((p, i) => <CasePoint key={i} point={p} color="green" />)}
          </div>
        )}
      </div>

      {/* Bear case */}
      <div style={{ borderTop: "1px solid #e2e8f0" }}>
        <button
          onClick={() => setShowBear(!showBear)}
          style={{
            width: "100%", padding: "14px 20px", border: "none", background: "rgba(239,68,68,0.04)",
            cursor: "pointer", display: "flex", alignItems: "center", justifyContent: "space-between",
            fontFamily: "'DM Sans', sans-serif",
          }}
        >
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <span style={{ fontSize: 14, fontWeight: 600, color: "#dc2626" }}>The Bear Case</span>
            <span style={{ fontSize: 12, color: "#94a3b8" }}>({bearPoints.length} factors)</span>
          </div>
          <span style={{
            fontSize: 18, color: "#94a3b8",
            transform: showBear ? "rotate(180deg)" : "rotate(0deg)", transition: "transform 0.2s",
          }}>▾</span>
        </button>
        {showBear && (
          <div style={{ padding: "0 20px 8px" }}>
            {bearPoints.map((p, i) => <CasePoint key={i} point={p} color="red" />)}
          </div>
        )}
      </div>

      {/* Verdict */}
      {verdict ? (
        <div style={{
          padding: "16px 20px", borderTop: "1px solid #e2e8f0",
          background: "rgba(99,102,241,0.04)",
        }}>
          <div style={{ fontSize: 12, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.3, textTransform: "uppercase", marginBottom: 10 }}>Our read</div>
          <div style={{ fontSize: 13, color: "#475569", lineHeight: 1.7 }}>
            <p style={{ margin: 0 }}>{verdict}</p>
          </div>
        </div>
      ) : (
        /* Fallback: HUL hardcoded verdict (when verdict prop is null/undefined) */
        <div style={{
          padding: "16px 20px", borderTop: "1px solid #e2e8f0",
          background: "rgba(99,102,241,0.04)",
        }}>
          <div style={{ fontSize: 12, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.3, textTransform: "uppercase", marginBottom: 10 }}>Our read</div>
          <div style={{ fontSize: 13, color: "#475569", lineHeight: 1.7 }}>
            <p style={{ margin: "0 0 10px" }}>
              HUL is doing the right things — new CEO, quick commerce pivot, premium acquisitions, ₹2,000 Cr capex.
              The playbook is textbook correct. The question is whether a company with 2% revenue growth can execute
              fast enough against D2C brands that iterate in weeks, not quarters.
            </p>
            <p style={{ margin: "0 0 10px" }}>
              The wildcard is Shikhar. With 1.4M retailers and a third of HUL's sales already flowing through it,
              this is quietly becoming India's largest FMCG distribution platform. If HUL opens it to third-party brands
              — earning platform fees on top of product margins — it shifts the story from &quot;slow-growth FMCG&quot;
              to &quot;infrastructure monopoly.&quot; That&apos;s a re-rating trigger the market hasn&apos;t priced in.
            </p>
            <p style={{ margin: "0 0 10px" }}>
              The bull case requires premiumisation, QC, and Shikhar to inflect growth to double-digit EPS within 2–3 years.
              The bear case says HUL has been promising pivots for years while earnings flatline and the P/E compresses.
            </p>
            <p style={{ margin: 0 }}>
              <strong style={{ color: "#1e293b" }}>To justify holding:</strong> HUL needs to deliver ~19% CAGR going forward just to match the Nifty 500 + 3% hurdle.
              That&apos;s a level of growth HUL hasn&apos;t achieved in any 5-year period in the last decade.
              The stock may be a great business — but that doesn&apos;t make it a great investment at this price.
            </p>
          </div>
        </div>
      )}
    </div>
  );
}
