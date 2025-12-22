// src/views/MainView.tsx
export function MainView() {
  return (
    <div className="p-8">
      <h1 className="text-3xl font-bold text-gray-900 mb-6">Dashboard</h1>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 mb-8">
        {/* Example stat cards - feel free to customize or remove */}
        <div className="bg-white rounded-lg shadow p-6">
          <h3 className="text-lg font-medium text-gray-600">Total Orders</h3>
          <p className="text-3xl font-bold text-indigo-600 mt-2">1,234</p>
        </div>
        <div className="bg-white rounded-lg shadow p-6">
          <h3 className="text-lg font-medium text-gray-600">Revenue</h3>
          <p className="text-3xl font-bold text-green-600 mt-2">$45,678</p>
        </div>
        <div className="bg-white rounded-lg shadow p-6">
          <h3 className="text-lg font-medium text-gray-600">Customers</h3>
          <p className="text-3xl font-bold text-blue-600 mt-2">892</p>
        </div>
      </div>

      <div className="bg-white rounded-lg shadow p-6">
        <h2 className="text-xl font-semibold text-gray-800 mb-4">
          Welcome back!
        </h2>
        <p className="text-gray-600">
          This is your main dashboard. Use the sidebar on the left to navigate
          to Chat or Admin sections.
        </p>
        <p className="text-gray-500 text-sm mt-4">
          You're currently viewing the Home route ("/").
        </p>
      </div>
    </div>
  );
}
