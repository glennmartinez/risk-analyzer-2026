import { Link } from "@tanstack/react-router";

export function Admin() {
  return (
    <div>
      <h1 className=" text-2xl text-destructive">Admin View</h1>

      <Link to="/settings" className=" text-blue-500 underline">
        Go to Settings
      </Link>

      <Link to="/" className=" text-blue-500 underline ml-4">
        Go to Home
      </Link>
    </div>
  );
}
