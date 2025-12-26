import { Bell, Search } from "lucide-react";

export function Header() {
  return (
    <header className="bg-white border-b border-gray-200 px-8 py-4 h-20 ">
      <div className="flex items-center justify-between">
        {/*<h1 className="text-2xl font-semibold text-gray-900">{section.title}</h1>*/}
        <div> </div>
        <div className="flex items-center gap-4">
          {/* User avatars */}
          <div className="flex items-center -space-x-2">
            <div className="w-8 h-8 rounded-full bg-linear-to-br from-pink-400 to-pink-600 border-2 border-white"></div>
            <div className="w-8 h-8 rounded-full bg-linear-to-br from-blue-400 to-blue-600 border-2 border-white"></div>
            <div className="text-sm text-gray-600 pl-3">+2</div>
            <button className="w-8 h-8 rounded-full border-2 border-dashed border-gray-300 flex items-center justify-center ml-2 hover:border-gray-400 transition-colors">
              <span className="text-gray-400 text-lg leading-none">+</span>
            </button>
          </div>

          {/* Notification bell */}
          <button className="relative p-2 hover:bg-gray-50 rounded-lg transition-colors">
            <Bell className="w-5 h-5 text-gray-600" />
            <span className="absolute top-1 right-1 w-2 h-2 bg-red-500 rounded-full"></span>
          </button>

          {/* Search */}
          <div className="flex items-center gap-2 bg-gray-100 rounded-lg px-4 py-2 min-w-[240px]">
            <Search className="w-4 h-4 text-gray-400" />
            <input
              type="text"
              placeholder="Search anything"
              className="bg-transparent border-none outline-none text-sm text-gray-900 placeholder-gray-400 flex-1"
            />
            <kbd className="text-xs text-gray-400 font-mono">⌘K</kbd>
          </div>

          {/* User profile */}
          <button className="flex items-center gap-2 hover:bg-gray-50 rounded-lg px-2 py-1 transition-colors">
            <div className="w-8 h-8 rounded-full bg-gradient-to-br from-purple-400 to-purple-600"></div>
          </button>
        </div>
      </div>
    </header>
  );
}
