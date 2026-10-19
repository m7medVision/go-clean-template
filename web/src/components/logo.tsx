import { ListChecksIcon } from "lucide-react";

import { cn } from "#/lib/utils";

export function Logo({ className }: { className?: string }) {
  return (
    <div className={cn("flex items-center gap-3", className)}>
      <span className="flex size-8 items-center justify-center rounded-full bg-primary text-primary-foreground">
        <ListChecksIcon className="size-4" />
      </span>
      <span className="text-xl font-bold">Go Clean Template</span>
    </div>
  );
}
