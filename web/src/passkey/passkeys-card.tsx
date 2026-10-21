import { useMutation, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { KeyRoundIcon, MoreHorizontalIcon, PencilIcon, PlusIcon, Trash2Icon } from "lucide-react";
import { useState } from "react";

import { FormError } from "#/components/form/form-components";
import { useAppForm, zodValidation } from "#/components/form/use-app-form";
import { Button } from "#/components/ui/button";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "#/components/ui/card";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "#/components/ui/dropdown-menu";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "#/components/ui/dialog";
import { isReauthRequired } from "#/auth/reauth";
import { profileQuery } from "#/user/api";
import { addPasskey, type Passkey, passkeysKey, passkeysQuery } from "./api";
import { DeletePasskeyDialog, RenamePasskeyDialog } from "./passkey-dialogs";
import { passkeySchema, reauthSchema } from "./schema";
import { passkeysSupported } from "./webauthn";

const formatDate = (iso: string) => new Date(iso).toLocaleDateString();

// PasskeysCard lists the user's passkeys, renames and deletes them and, where the browser can,
// adds one.
export function PasskeysCard() {
  const { data } = useSuspenseQuery(passkeysQuery);
  const [adding, setAdding] = useState(false);
  const [renaming, setRenaming] = useState<Passkey>();
  const [deleting, setDeleting] = useState<Passkey>();

  return (
    <Card>
      <CardHeader>
        <CardTitle>Passkeys</CardTitle>
        <CardDescription>Sign in with your fingerprint, face or device PIN.</CardDescription>
        {passkeysSupported() && (
          <CardAction>
            <Button variant="outline" size="sm" onClick={() => setAdding(true)}>
              <PlusIcon />
              Add a passkey
            </Button>
          </CardAction>
        )}
      </CardHeader>
      <CardContent>
        {data.passkeys.length === 0 ? (
          <p className="text-sm text-muted-foreground">No passkeys yet.</p>
        ) : (
          <ul className="divide-y text-sm">
            {data.passkeys.map((passkey) => (
              <li key={passkey.id} className="flex items-center gap-3 py-3 first:pt-0 last:pb-0">
                <KeyRoundIcon className="size-4 shrink-0 text-muted-foreground" aria-hidden />
                <div className="min-w-0 flex-1">
                  <p className="truncate font-medium">{passkey.name}</p>
                  <p className="text-muted-foreground">
                    Added {formatDate(passkey.created_at)} · Last used{" "}
                    {passkey.last_used_at ? formatDate(passkey.last_used_at) : "never"}
                  </p>
                </div>
                <PasskeyActions passkey={passkey} onRename={setRenaming} onDelete={setDeleting} />
              </li>
            ))}
          </ul>
        )}
      </CardContent>
      <AddPasskeyDialog open={adding} onOpenChange={setAdding} />
      <RenamePasskeyDialog
        passkey={renaming}
        onOpenChange={(open) => !open && setRenaming(undefined)}
      />
      <DeletePasskeyDialog
        passkey={deleting}
        onOpenChange={(open) => !open && setDeleting(undefined)}
      />
    </Card>
  );
}

function PasskeyActions({
  passkey,
  onRename,
  onDelete,
}: {
  passkey: Passkey;
  onRename: (passkey: Passkey) => void;
  onDelete: (passkey: Passkey) => void;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label={`Actions for ${passkey.name}`}>
          <MoreHorizontalIcon />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onSelect={() => onRename(passkey)}>
          <PencilIcon />
          Rename
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive" onSelect={() => onDelete(passkey)}>
          <Trash2Icon />
          Delete
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function AddPasskeyDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        {/* Mounted only while open, so each opening starts with an empty form. */}
        {open && <AddPasskeyForm onDone={() => onOpenChange(false)} />}
      </DialogContent>
    </Dialog>
  );
}

function AddPasskeyForm({ onDone }: { onDone: () => void }) {
  const queryClient = useQueryClient();
  const { data: user } = useSuspenseQuery(profileQuery);
  // Set once the server asks for a recent sign-in: the name already chosen, kept for the retry.
  const [reauthFor, setReauthFor] = useState<{ name?: string }>();
  const add = useMutation({
    ...addPasskey,
    onSuccess: (passkey) => {
      // null: the user dismissed the browser's prompt. Stay open and say nothing.
      if (!passkey) return;

      void queryClient.invalidateQueries({ queryKey: passkeysKey });
      onDone();
    },
    onError: (error, { name }) => {
      if (isReauthRequired(error)) setReauthFor({ name });
    },
  });

  if (reauthFor) {
    return (
      <ReauthForm
        email={user.email}
        error={isReauthRequired(add.error) ? null : add.error}
        onSubmit={(password) =>
          add.mutateAsync({ ...reauthFor, reauth: { email: user.email, password } })
        }
      />
    );
  }

  return <NameForm error={add.error} onSubmit={(value) => add.mutateAsync(value)} />;
}

function NameForm({
  error,
  onSubmit,
}: {
  error: unknown;
  onSubmit: (value: { name?: string }) => Promise<unknown>;
}) {
  const form = useAppForm({
    defaultValues: { name: "" },
    ...zodValidation(passkeySchema),
    onSubmit: ({ value }) => onSubmit(passkeySchema.parse(value)),
  });

  return (
    <>
      <DialogHeader>
        <DialogTitle>Add a passkey</DialogTitle>
        <DialogDescription>
          Your browser will ask you to confirm with this device or a security key.
        </DialogDescription>
      </DialogHeader>
      <form.AppForm>
        <form.Form>
          <form.AppField name="name">
            {(field) => (
              <field.TextField
                label="Name"
                description="Optional. Helps you tell your passkeys apart."
                placeholder="MacBook Touch ID"
                autoComplete="off"
              />
            )}
          </form.AppField>
          <FormError error={error} />
          <DialogFooter className="mt-2">
            <DialogClose asChild>
              <Button type="button" variant="outline">
                Cancel
              </Button>
            </DialogClose>
            <form.SubmitButton pending="Waiting for your device…">Continue</form.SubmitButton>
          </DialogFooter>
        </form.Form>
      </form.AppForm>
    </>
  );
}

// ReauthForm asks for the password when the sign-in is too old to add a passkey; submitting signs
// in again and continues adding the passkey.
function ReauthForm({
  email,
  error,
  onSubmit,
}: {
  email: string;
  error: unknown;
  onSubmit: (password: string) => Promise<unknown>;
}) {
  const form = useAppForm({
    defaultValues: { password: "" },
    ...zodValidation(reauthSchema),
    onSubmit: ({ value }) => onSubmit(reauthSchema.parse(value).password),
  });

  return (
    <>
      <DialogHeader>
        <DialogTitle>Confirm it’s you</DialogTitle>
        <DialogDescription>
          Enter the password for {email} to add a passkey. Then your browser will ask you to confirm
          with this device or a security key.
        </DialogDescription>
      </DialogHeader>
      <form.AppForm>
        <form.Form>
          <form.AppField name="password">
            {(field) => (
              <field.PasswordField
                label="Password"
                autoComplete="current-password"
                placeholder="••••••••••••"
              />
            )}
          </form.AppField>
          <FormError error={error} />
          <DialogFooter className="mt-2">
            <DialogClose asChild>
              <Button type="button" variant="outline">
                Cancel
              </Button>
            </DialogClose>
            <form.SubmitButton pending="Waiting for your device…">Continue</form.SubmitButton>
          </DialogFooter>
        </form.Form>
      </form.AppForm>
    </>
  );
}
