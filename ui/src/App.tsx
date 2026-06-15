import { Routes, Route, Link } from "react-router-dom";
import Dashboard from "./pages/Dashboard";
import RunDetail from "./pages/RunDetail";
import AttemptDetail from "./pages/AttemptDetail";
import Features from "./pages/Features";

export default function App() {
  return (
    <>
      <header className="top">
        <div className="container">
          <h1>
            <Link to="/">pave</Link>
          </h1>
          <nav>
            <Link to="/">Dashboard</Link>
            <Link to="/features">Features</Link>
          </nav>
        </div>
      </header>
      <main className="container">
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/runs/:id" element={<RunDetail />} />
          <Route path="/runs/:id/attempts/:attemptId" element={<AttemptDetail />} />
          <Route path="/features" element={<Features />} />
        </Routes>
      </main>
    </>
  );
}
