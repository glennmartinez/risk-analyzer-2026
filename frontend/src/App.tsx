import "./App.css";
import { Outlet } from "@tanstack/react-router";
import { TanStackRouterDevtools } from "@tanstack/react-router-devtools";
import { Sidebar } from "./components/Sidebar";
import { Header } from "./components/Header";

function App() {
  return (
    <div className="min-h-screen bg-[#F5F6FA]">
      {/* Your header from earlier, if any */}
      <div className="flex">
        <Sidebar /> {/* No props needed! */}
        <main className="flex-1 ml-64">
          <Header /> {/* Adjust ml- for sidebar width */}
          <Outlet /> {/* Renders the matched route component */}
        </main>
      </div>
      <TanStackRouterDevtools />
    </div>
  );
}

export default App;
