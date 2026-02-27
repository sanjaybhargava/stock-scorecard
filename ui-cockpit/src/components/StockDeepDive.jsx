import PhasePerformance from "./cards/PhasePerformance.jsx";
import MaximumPain from "./cards/MaximumPain.jsx";
import CoveredCall from "./cards/CoveredCall.jsx";
import RedeploymentPlan from "./cards/RedeploymentPlan.jsx";
import FutureProspects from "./cards/FutureProspects.jsx";
import TerminalValue from "./cards/TerminalValue.jsx";

// HUL-specific data for card 3 (Covered Call — only HUL has F&O data)
import {
  HUL_STRATEGY,
  HUL_BACKTEST,
  HUL_DEFICIT,
  HUL_ANNUAL,
  HUL_ITM_EVENTS,
  HUL_BULL_POINTS,
  HUL_BEAR_POINTS,
} from "../data/hulCards.js";

// Bull/bear prospects for all other failed stocks
import PROSPECTS from "../data/stockProspects.js";

export default function StockDeepDive({ stock, data, onBack }) {
  const isHUL = stock.symbol === "HINDUNILVR";
  const cards = stock.cards;

  if (!cards) {
    return (
      <div>
        <BackButton onClick={onBack} />
        <div style={{
          padding: "40px 20px", textAlign: "center",
          background: "#fff", borderRadius: 12, border: "1px solid #e2e8f0",
        }}>
          <div style={{ fontSize: 16, fontWeight: 600, color: "#475569" }}>No deep-dive data available</div>
        </div>
      </div>
    );
  }

  // Get bull/bear points: HUL from hulCards.js, others from stockProspects.js
  const prospects = isHUL
    ? { bull: HUL_BULL_POINTS, bear: HUL_BEAR_POINTS, verdict: null }
    : PROSPECTS[stock.symbol] || null;

  const hasProspects = !!prospects;
  const cardCount = (isHUL ? 6 : 4) + (hasProspects && !isHUL ? 1 : 0);

  return (
    <div>
      <BackButton onClick={onBack} />

      {/* Header */}
      <div style={{ marginBottom: 24 }}>
        <div style={{ fontSize: 11, fontWeight: 600, color: "#94a3b8", letterSpacing: 0.5, textTransform: "uppercase", marginBottom: 4 }}>
          DEEP DIVE
        </div>
        <h1 style={{ fontSize: 24, fontWeight: 700, color: "#0f172a", margin: 0 }}>
          {stock.name}
        </h1>
        <p style={{ fontSize: 14, color: "#64748b", margin: "8px 0 0" }}>
          {cardCount} decision cards to evaluate this holding
        </p>
      </div>

      <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
        {/* Card 1: Phase Performance — all stocks */}
        <PhasePerformance stockName={stock.name} phases={cards.phases} />

        {/* Card 2: Maximum Pain — all stocks */}
        <MaximumPain stockName={stock.name} phases={cards.drawdowns} />

        {/* Card 3: Covered Call — HUL only */}
        {isHUL && (
          <CoveredCall
            strategy={HUL_STRATEGY}
            backtest={HUL_BACKTEST}
            deficit={HUL_DEFICIT}
            annual={HUL_ANNUAL}
            itmEvents={HUL_ITM_EVENTS}
          />
        )}

        {/* Card 4: Redeployment Plan — all stocks */}
        <RedeploymentPlan stock={cards.redeployment} />

        {/* Card 5: Future Prospects — all stocks with prospect data */}
        {hasProspects && (
          <FutureProspects
            stockName={stock.name}
            bullPoints={prospects.bull}
            bearPoints={prospects.bear}
            verdict={prospects.verdict}
          />
        )}

        {/* Card 6: Terminal Value — all stocks */}
        <TerminalValue stock={cards.terminal} />
      </div>
    </div>
  );
}

function BackButton({ onClick }) {
  return (
    <button
      onClick={onClick}
      style={{
        background: "none", border: "none", cursor: "pointer", fontSize: 13,
        color: "#6366f1", fontWeight: 500, fontFamily: "'DM Sans', sans-serif",
        padding: "0 0 16px", display: "flex", alignItems: "center", gap: 4,
      }}
    >
      ← Back to test results
    </button>
  );
}
