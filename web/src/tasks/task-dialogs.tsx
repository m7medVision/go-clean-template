import { useMutation, useQueryClient } from "@tanstack/react-query";

import { problemMessage } from "#/api/problem";
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
import { FormError } from "#/components/form/form-components";
import { useAppForm, zodValidation } from "#/components/form/use-app-form";
import { createTask, deleteTask, tasksKey, updateTask } from "./api";
import { taskSchema } from "./schema";
import type { Task } from "./status";

// TaskFormDialog creates a task, or edits `task` when one is given.
export function TaskFormDialog({
  task,
  open,
  onOpenChange,
}: {
  task?: Task;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        {/* Keyed so each opening starts from the task's current values. */}
        {open && (
          <TaskForm key={task?.id ?? "new"} task={task} onDone={() => onOpenChange(false)} />
        )}
      </DialogContent>
    </Dialog>
  );
}

function TaskForm({ task, onDone }: { task?: Task; onDone: () => void }) {
  const queryClient = useQueryClient();

  const onSuccess = () => {
    void queryClient.invalidateQueries({ queryKey: tasksKey });
    onDone();
  };
  const create = useMutation({ ...createTask, onSuccess });
  const update = useMutation({ ...updateTask, onSuccess });

  const form = useAppForm({
    defaultValues: { title: task?.title ?? "", description: task?.description ?? "" },
    ...zodValidation(taskSchema),
    onSubmit: ({ value }) => {
      const input = taskSchema.parse(value);

      return task ? update.mutateAsync({ id: task.id, input }) : create.mutateAsync({ input });
    },
  });

  return (
    <>
      <DialogHeader>
        <DialogTitle>{task ? "Edit task" : "Add task"}</DialogTitle>
        <DialogDescription>
          {task ? "Change the title or description." : "New tasks start in To do."}
        </DialogDescription>
      </DialogHeader>
      <form.AppForm>
        <form.Form>
          <form.AppField name="title">
            {(field) => <field.TextField label="Title" required placeholder="What needs doing?" />}
          </form.AppField>
          <form.AppField name="description">
            {(field) => (
              <field.TextareaField
                label="Description"
                rows={4}
                placeholder="Add details (optional)"
              />
            )}
          </form.AppField>
          <FormError error={(task ? update : create).error} />
          <DialogFooter className="mt-2">
            <DialogClose asChild>
              <Button type="button" variant="outline">
                Cancel
              </Button>
            </DialogClose>
            <form.SubmitButton pending="Saving…">
              {task ? "Save changes" : "Add task"}
            </form.SubmitButton>
          </DialogFooter>
        </form.Form>
      </form.AppForm>
    </>
  );
}

export function DeleteTaskDialog({
  task,
  onOpenChange,
}: {
  task: Task | undefined;
  onOpenChange: (open: boolean) => void;
}) {
  const queryClient = useQueryClient();
  const remove = useMutation({
    ...deleteTask,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: tasksKey });
      onOpenChange(false);
    },
  });

  return (
    <AlertDialog
      open={task !== undefined}
      onOpenChange={(open) => {
        if (!open) remove.reset();
        onOpenChange(open);
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete this task?</AlertDialogTitle>
          <AlertDialogDescription>
            “{task?.title}” will be deleted permanently. You can’t undo this.
          </AlertDialogDescription>
        </AlertDialogHeader>
        {remove.isError && (
          <p role="alert" className="text-sm text-destructive">
            {problemMessage(remove.error)}
          </p>
        )}
        <AlertDialogFooter>
          <AlertDialogCancel>Keep task</AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            disabled={remove.isPending}
            onClick={(event) => {
              // Stay open until the delete lands, so a failure can be shown.
              event.preventDefault();
              if (task) remove.mutate({ id: task.id });
            }}
          >
            {remove.isPending ? "Deleting…" : "Delete task"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
