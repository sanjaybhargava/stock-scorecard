import { useState, useEffect, useRef } from "react";
import PortfolioSummary from "./components/PortfolioSummary.jsx";
import TestResults from "./components/TestResults.jsx";
import StockDeepDive from "./components/StockDeepDive.jsx";

export default function App() {
  const [data, setData] = useState(null);
  const [error, setError] = useState(null);
  const [layer, setLayer] = useState(1);
  const [selectedStock, setSelectedStock] = useState(null);
  const topRef = useRef(null);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const clientId = params.get("client");

    if (!clientId) {
      setError("Add ?client=ZY7393 to the URL");
      return;
    }

    fetch(`cockpit_${clientId}.json`)
      .then((r) => {
        if (!r.ok) throw new Error(`No data found for client ${clientId}`);
        return r.json();
      })
      .then(setData)
      .catch((e) => setError(e.message));
  }, []);

  function navigate(newLayer, stock) {
    setLayer(newLayer);
    setSelectedStock(stock || null);
    window.scrollTo({ top: 0, behavior: "smooth" });
  }

  if (error) {
    return (
      <div style={{ minHeight: "100vh", display: "flex", alignItems: "center", justifyContent: "center", fontFamily: "'DM Sans', sans-serif" }}>
        <div style={{ textAlign: "center" }}>
          <div style={{ fontSize: 18, fontWeight: 600, color: "#0f172a", marginBottom: 8 }}>Portfolio Cockpit</div>
          <div style={{ fontSize: 14, color: "#dc2626" }}>{error}</div>
        </div>
      </div>
    );
  }

  if (!data) {
    return (
      <div style={{ minHeight: "100vh", display: "flex", alignItems: "center", justifyContent: "center", fontFamily: "'DM Sans', sans-serif" }}>
        <div style={{ fontSize: 16, color: "#64748b" }}>Loading...</div>
      </div>
    );
  }

  return (
    <div ref={topRef} style={{ minHeight: "100vh", background: "#f8fafc", fontFamily: "'DM Sans', sans-serif", padding: "32px 20px" }}>
      <div style={{ maxWidth: 700, margin: "0 auto" }}>
        {layer === 1 && (
          <PortfolioSummary data={data} onNavigate={() => navigate(2)} />
        )}
        {layer === 2 && (
          <TestResults
            data={data}
            onBack={() => navigate(1)}
            onStockClick={(stock) => navigate(3, stock)}
          />
        )}
        {layer === 3 && selectedStock && (
          <StockDeepDive
            stock={selectedStock}
            data={data}
            onBack={() => navigate(2)}
          />
        )}
      </div>
    </div>
  );
}
