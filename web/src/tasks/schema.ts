import { z } from "zod";

// Mirrors the GraphQL resolver's validTaskInput; the server stays the authority.
const _maxTitle = 255;
const _maxDescription = 1000;

// taskSchema validates both creating and editing a task (CreateTaskInput / UpdateTaskInput).
export const taskSchema = z.object({
  title: z
    .string()
    .trim()
    .min(1, "Give the task a title.")
    .max(_maxTitle, `Keep the title under ${_maxTitle} characters.`),
  description: z
    .string()
    .trim()
    .max(_maxDescription, `Keep the description under ${_maxDescription} characters.`),
});

// Mirrors the attribute rules in internal/entity/mongo/task; the server stays the authority.
export const priorities = ["LOW", "MEDIUM", "HIGH"] as const;
export type Priority = (typeof priorities)[number];
export const noPriority = "NONE";

const _minEstimate = 0.5;
const _maxEstimate = 200;
const _maxLabels = 10;
const _maxLabel = 30;

const splitLabels = (value: string) =>
  value
    .split(",")
    .map((label) => label.trim())
    .filter((label) => label !== "");

export const attributesSchema = z.object({
  priority: z.enum([noPriority, ...priorities]),
  estimate: z
    .string()
    .trim()
    .refine(
      (value) =>
        value === "" ||
        (Number.isFinite(Number(value)) &&
          Number(value) >= _minEstimate &&
          Number(value) <= _maxEstimate),
      `Estimate between ${_minEstimate} and ${_maxEstimate} hours.`,
    ),
  labels: z
    .string()
    .refine((value) => splitLabels(value).length <= _maxLabels, `At most ${_maxLabels} labels.`)
    .refine(
      (value) => splitLabels(value).every((label) => label.length <= _maxLabel),
      `Keep each label under ${_maxLabel} characters.`,
    )
    .refine(
      (value) => new Set(splitLabels(value)).size === splitLabels(value).length,
      "Use each label once.",
    ),
});

export const editTaskSchema = taskSchema.extend(attributesSchema.shape);

export type AttributesForm = z.infer<typeof attributesSchema>;

export type Attributes = {
  priority?: Priority | null;
  estimate?: number | null;
  labels?: string[] | null;
};

export function attributesForm(attributes: Attributes): AttributesForm {
  return {
    priority: attributes.priority ?? noPriority,
    estimate: attributes.estimate == null ? "" : String(attributes.estimate),
    labels: (attributes.labels ?? []).join(", "),
  };
}

// attributesPatch is what the form changed: only the fields that differ from `current`, with null
// for a field the user emptied, so the PATCH leaves the rest alone.
export function attributesPatch(current: Attributes, form: AttributesForm): Attributes {
  const patch: Attributes = {};

  const priority = form.priority === noPriority ? null : form.priority;
  if (priority !== (current.priority ?? null)) patch.priority = priority;

  const estimate = form.estimate.trim() === "" ? null : Number(form.estimate);
  if (estimate !== (current.estimate ?? null)) patch.estimate = estimate;

  const labels = splitLabels(form.labels);
  const before = current.labels ?? [];
  if (labels.join("\n") !== before.join("\n")) patch.labels = labels.length > 0 ? labels : null;

  return patch;
}
