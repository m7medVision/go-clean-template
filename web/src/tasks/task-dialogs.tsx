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
import { createTask, deleteTask, tasksKey, updateTask, updateTaskAttributes } from "./api";
import {
  attributesForm,
  attributesPatch,
  editTaskSchema,
  noPriority,
  priorities,
  taskSchema,
} from "./schema";
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
  return task ? <EditTaskForm task={task} onDone={onDone} /> : <CreateTaskForm onDone={onDone} />;
}

function CreateTaskForm({ onDone }: { onDone: () => void }) {
  const queryClient = useQueryClient();
  const create = useMutation({
    ...createTask,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: tasksKey });
      onDone();
    },
  });

  const form = useAppForm({
    defaultValues: { title: "", description: "" },
    ...zodValidation(taskSchema),
    onSubmit: ({ value }) => create.mutateAsync({ input: taskSchema.parse(value) }),
  });

  return (
    <>
      <DialogHeader>
        <DialogTitle>Add task</DialogTitle>
        <DialogDescription>New tasks start in To do.</DialogDescription>
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
          <FormError error={create.error} />
          <DialogFooter className="mt-2">
            <DialogClose asChild>
              <Button type="button" variant="outline">
                Cancel
              </Button>
            </DialogClose>
            <form.SubmitButton pending="Saving…">Add task</form.SubmitButton>
          </DialogFooter>
        </form.Form>
      </form.AppForm>
    </>
  );
}

const priorityOptions = [
  { value: noPriority, label: "None" },
  ...priorities.map((p) => ({ value: p, label: p[0] + p.slice(1).toLowerCase() })),
];

function EditTaskForm({ task, onDone }: { task: Task; onDone: () => void }) {
  const queryClient = useQueryClient();
  const update = useMutation(updateTask);
  const updateAttributes = useMutation(updateTaskAttributes);

  const form = useAppForm({
    defaultValues: {
      title: task.title,
      description: task.description,
      ...attributesForm(task.attributes),
    },
    ...zodValidation(editTaskSchema),
    onSubmit: async ({ value }) => {
      const { title, description } = taskSchema.parse(value);
      await update.mutateAsync({ id: task.id, input: { title, description } });

      const patch = attributesPatch(task.attributes, value);
      if (Object.keys(patch).length > 0) {
        await updateAttributes.mutateAsync({ id: task.id, input: patch });
      }

      void queryClient.invalidateQueries({ queryKey: tasksKey });
      onDone();
    },
  });

  return (
    <>
      <DialogHeader>
        <DialogTitle>Edit task</DialogTitle>
        <DialogDescription>Change the title, the description or the attributes.</DialogDescription>
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
          <form.AppField name="priority">
            {(field) => <field.SelectField label="Priority" options={priorityOptions} />}
          </form.AppField>
          <form.AppField name="estimate">
            {(field) => (
              <field.TextField label="Estimate" description="Hours, 0.5–200." inputMode="decimal" />
            )}
          </form.AppField>
          <form.AppField name="labels">
            {(field) => <field.TextField label="Labels" description="Separated by commas." />}
          </form.AppField>
          <FormError error={update.error ?? updateAttributes.error} />
          <DialogFooter className="mt-2">
            <DialogClose asChild>
              <Button type="button" variant="outline">
                Cancel
              </Button>
            </DialogClose>
            <form.SubmitButton pending="Saving…">Save changes</form.SubmitButton>
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
