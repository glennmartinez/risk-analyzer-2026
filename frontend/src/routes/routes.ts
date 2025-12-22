import {
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import App from "../App";
import { MainView } from "../views/MainView";
import { Admin } from "../views/Admin";
import ChatView from "../views/ChatView";

const rootRoute = createRootRoute({
  component: App,
});

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: MainView,
});

const chatRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/chat",
  component: ChatView,
});

const adminRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/admin",
  component: Admin,
});

// Building the route tree
const routeTree = rootRoute.addChildren([indexRoute, chatRoute, adminRoute]);

export const Router = createRouter({
  routeTree,
  defaultPreload: "intent",
});
