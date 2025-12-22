import { Link, useLocation } from "@tanstack/react-router";
import {
  LayoutDashboard,
  MessageSquare,
  PersonStandingIcon,
  Shield,
  TestTube,
} from "lucide-react";

export function Sidebar() {
  const location = useLocation();

  const isActive = (path: string) => location.pathname === path;

  return (
    <aside className="fixed left-0 top-0 h-screen w-64 bg-white border-r border-gray-200 flex flex-col">
      {/* Logo/Header */}
      <div className="p-6 border-b border-gray-200">
        <div className="flex items-center gap-3">
          <PersonStandingIcon className="w-8 h-8 text-blue-600" />
          <span className="text-lg font-semibold text-gray-900">
            Risk Analyzer
          </span>
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex-1 px-4 py-6 space-y-1">
        <Link
          to="/"
          className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-colors ${
            isActive("/")
              ? "bg-gray-100 text-gray-900 font-medium"
              : "text-gray-600 hover:bg-gray-50 hover:text-gray-900"
          }`}
        >
          <LayoutDashboard className="w-5 h-5" />
          <span>Home</span>
        </Link>

        <Link
          to="/chat"
          className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-colors ${
            isActive("/chat")
              ? "bg-gray-100 text-gray-900 font-medium"
              : "text-gray-600 hover:bg-gray-50 hover:text-gray-900"
          }`}
        >
          <MessageSquare className="w-5 h-5" />
          <span>Chat</span>
        </Link>

        <Link
          to="/admin"
          className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-colors ${
            isActive("/admin")
              ? "bg-gray-100 text-gray-900 font-medium"
              : "text-gray-600 hover:bg-gray-50 hover:text-gray-900"
          }`}
        >
          <Shield className="w-5 h-5" />
          <span>Admin</span>
        </Link>

        <Link
          to="/rag"
          className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-colors ${
            isActive("/rag")
              ? "bg-gray-100 text-gray-900 font-medium"
              : "text-gray-600 hover:bg-gray-50 hover:text-gray-900"
          }`}
        >
          <TestTube className="w-5 h-5 " />
          <span>Rag Analysis</span>
        </Link>
      </nav>

      {/* Optional bottom section (e.g., user, dark mode, etc.) */}
      <div className="p-4 border-t border-gray-200">
        <div className="flex items-center gap-3 p-3 bg-gray-50 rounded-lg">
          <div className="w-8 h-8 bg-black rounded-lg flex items-center justify-center">
            <div className="w-4 h-4 bg-white rounded-sm"></div>
          </div>
          <span className="text-sm font-medium text-gray-900">Uxerflow</span>
        </div>
      </div>
    </aside>
  );
}
