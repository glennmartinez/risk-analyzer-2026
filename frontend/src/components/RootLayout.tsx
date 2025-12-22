import { Link, Outlet } from "@tanstack/react-router";
import "./RootLayout.css"; // Optional styling

export function RootLayout() {
  return (
    <div className="min-h-screen bg-gray-50">
      {/* Shared Header / Navigation */}
      <header className="bg-white shadow-sm border-b">
        <div className="max-w-7xl mx-auto px-4 py-4 flex items-center gap-6">
          <h1 className="text-xl font-semibold">My App</h1>
          <nav className="flex gap-6">
            <Link
              to="/home"
              activeProps={{ className: "text-blue-600 font-medium" }}
              className="text-gray-600 hover:text-blue-600"
            >
              Home
            </Link>
            <Link
              to="/chat"
              activeProps={{ className: "text-blue-600 font-medium" }}
              className="text-gray-600 hover:text-blue-600"
            >
              Chat
            </Link>
            <Link
              to="/admin"
              activeProps={{ className: "text-blue-600 font-medium" }}
              className="text-gray-600 hover:text-blue-600"
            >
              Admin
            </Link>
          </nav>
        </div>
      </header>

      {/* Page Content */}
      <main className="max-w-7xl mx-auto px-4 py-8">
        <Outlet />
      </main>
    </div>
  );
}
