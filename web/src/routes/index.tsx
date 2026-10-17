import { Link, createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/")({ component: Home });

function Home() {
  return (
    <section className="space-y-4">
      <h1 className="text-2xl font-bold">Go Clean Template</h1>
      <p>A React client for the REST API: users, tasks and translation.</p>
      <Link to="/profile" className="underline">
        Go to your profile
      </Link>
    </section>
  );
}
