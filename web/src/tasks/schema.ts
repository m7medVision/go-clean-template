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
