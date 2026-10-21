import { Fragment } from "react";
import { Link, Outlet, createFileRoute, redirect, useMatches } from "@tanstack/react-router";

import { AppSidebar } from "#/components/app-sidebar";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "#/components/ui/breadcrumb";
import { Separator } from "#/components/ui/separator";
import { SidebarInset, SidebarProvider, SidebarTrigger } from "#/components/ui/sidebar";

// Layout route for every screen that needs a signed-in user. The API is the real check:
// an expired token still reaches it, gets a 401, and the client sends the user back here.
export const Route = createFileRoute("/_authed")({
  beforeLoad: ({ context, location }) => {
    if (!context.auth.token()) {
      throw redirect({ to: "/login", search: { redirect: location.href } });
    }
  },
  component: AuthedLayout,
});

function AuthedLayout() {
  const { auth, queryClient } = Route.useRouteContext();
  const navigate = Route.useNavigate();

  // Leave the signed-in layout before clearing the cache: clearing it first lets the sidebar's
  // mounted profile query refetch without a token, and its 401 bounces the user to /login again.
  const logout = async () => {
    auth.clear();
    await navigate({ to: "/login" });
    queryClient.clear();
  };

  return (
    <SidebarProvider>
      <AppSidebar onLogout={logout} />
      <SidebarInset>
        <header className="flex h-14 shrink-0 items-center gap-2 border-b px-4">
          <SidebarTrigger className="-ml-1" />
          <Separator
            orientation="vertical"
            className="mr-2 data-vertical:h-4 data-vertical:self-center"
          />
          <Crumbs />
        </header>
        <div className="flex-1 p-4 md:p-8">
          <Outlet />
        </div>
      </SidebarInset>
    </SidebarProvider>
  );
}

// Crumbs builds the trail from the `title` each matched route declares in staticData.
function Crumbs() {
  const crumbs = useMatches({
    select: (matches) =>
      matches.flatMap((match) =>
        match.staticData.title ? [{ id: match.id, title: match.staticData.title }] : [],
      ),
  });

  return (
    <Breadcrumb>
      <BreadcrumbList>
        <BreadcrumbItem className="hidden md:block">
          <BreadcrumbLink asChild>
            <Link to="/tasks">Go Clean Template</Link>
          </BreadcrumbLink>
        </BreadcrumbItem>
        {crumbs.map((crumb, index) => (
          <Fragment key={crumb.id}>
            <BreadcrumbSeparator className="hidden md:block" />
            <BreadcrumbItem>
              {index === crumbs.length - 1 ? (
                <BreadcrumbPage>{crumb.title}</BreadcrumbPage>
              ) : (
                crumb.title
              )}
            </BreadcrumbItem>
          </Fragment>
        ))}
      </BreadcrumbList>
    </Breadcrumb>
  );
}
