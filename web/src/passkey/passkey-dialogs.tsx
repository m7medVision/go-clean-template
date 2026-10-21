import { useMutation, useQueryClient } from "@tanstack/react-query";

import { FormError } from "#/components/form/form-components";
import { useAppForm, zodValidation } from "#/components/form/use-app-form";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "#/components/ui/alert-dialog";
import { Button } from "#/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "#/components/ui/dialog";
import { deletePasskey, type Passkey, passkeysKey, renamePasskey } from "./api";
import { renamePasskeySchema } from "./schema";

// RenamePasskeyDialog renames `passkey`; it is open while one is given.
export function RenamePasskeyDialog({
  passkey,
  onOpenChange,
}: {
  passkey: Passkey | undefined;
  onOpenChange: (open: boolean) => void;
}) {
  return (
    <Dialog open={passkey !== undefined} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        {/* Keyed so each opening starts from the passkey's current name. */}
        {passkey && (
          <RenamePasskeyForm
            key={passkey.id}
            passkey={passkey}
            onDone={() => onOpenChange(false)}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function RenamePasskeyForm({ passkey, onDone }: { passkey: Passkey; onDone: () => void }) {
  const queryClient = useQueryClient();
  const rename = useMutation({
    ...renamePasskey,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: passkeysKey });
      onDone();
    },
  });

  const form = useAppForm({
    defaultValues: { name: passkey.name },
    ...zodValidation(renamePasskeySchema),
    onSubmit: ({ value }) =>
      rename.mutateAsync({ id: passkey.id, ...renamePasskeySchema.parse(value) }),
  });

  return (
    <>
      <DialogHeader>
        <DialogTitle>Rename passkey</DialogTitle>
        <DialogDescription>Pick a name that helps you tell your passkeys apart.</DialogDescription>
      </DialogHeader>
      <form.AppForm>
        <form.Form>
          <form.AppField name="name">
            {(field) => (
              <field.TextField label="Name" placeholder="MacBook Touch ID" autoComplete="off" />
            )}
          </form.AppField>
          <FormError error={rename.error} />
          <DialogFooter className="mt-2">
            <DialogClose asChild>
              <Button type="button" variant="outline">
                Cancel
              </Button>
            </DialogClose>
            <form.SubmitButton pending="Saving…">Save name</form.SubmitButton>
          </DialogFooter>
        </form.Form>
      </form.AppForm>
    </>
  );
}

// DeletePasskeyDialog asks before deleting `passkey`; it is open while one is given.
export function DeletePasskeyDialog({
  passkey,
  onOpenChange,
}: {
  passkey: Passkey | undefined;
  onOpenChange: (open: boolean) => void;
}) {
  const queryClient = useQueryClient();
  const remove = useMutation({
    ...deletePasskey,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: passkeysKey });
      onOpenChange(false);
    },
  });

  return (
    <AlertDialog
      open={passkey !== undefined}
      onOpenChange={(open) => {
        if (!open) remove.reset();
        onOpenChange(open);
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete this passkey?</AlertDialogTitle>
          <AlertDialogDescription>
            You won’t be able to sign in with “{passkey?.name}” any more. You can still sign in with
            your password, and add the passkey again later.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <FormError error={remove.error} />
        <AlertDialogFooter>
          <AlertDialogCancel>Keep passkey</AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            disabled={remove.isPending}
            onClick={(event) => {
              // Stay open until the delete lands, so a failure can be shown.
              event.preventDefault();
              if (passkey) remove.mutate({ id: passkey.id });
            }}
          >
            {remove.isPending ? "Deleting…" : "Delete passkey"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
