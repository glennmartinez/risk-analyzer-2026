import { Outlet } from "@tanstack/react-router";
import { TanStackRouterDevtools } from "@tanstack/react-router-devtools";
import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import { Sidebar } from "lucide-react";
import { Header } from "./Header";

export function Layout() {
  return (
    <div className=" ">
      <Sidebar />
      <div className="ml-64 flex-1 flex flex-col">
        <Header />
        <div className="flex-1 p-8 overflow-y-auto">
          <Outlet />
        </div>
      </div>

      <TanStackRouterDevtools />
      <ReactQueryDevtools initialIsOpen={false} />
    </div>
  );
}
