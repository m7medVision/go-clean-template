import { graphqlKey, graphqlMutationOptions, graphqlQueryOptions } from "#/api/graphql";
import { graphql } from "#/graphql";
import type { AllTasksQueryVariables, TasksQueryVariables } from "#/graphql/graphql";

// Every task read and write goes through GraphQL: the list asks only for the columns it shows.

const TasksQuery = graphql(`
  query Tasks($status: TaskStatus, $limit: Int!, $offset: Int!) {
    tasks(status: $status, limit: $limit, offset: $offset) {
      total
      tasks {
        id
        title
        description
        status
        createdAt
        updatedAt
      }
    }
  }
`);

// Every user's tasks, for admins: the columns the "All tasks" table shows, and the owner.
const AllTasksQuery = graphql(`
  query AllTasks($status: TaskStatus, $limit: Int!, $offset: Int!) {
    allTasks(status: $status, limit: $limit, offset: $offset) {
      total
      tasks {
        id
        userId
        title
        status
        createdAt
      }
    }
  }
`);

const CreateTaskMutation = graphql(`
  mutation CreateTask($input: CreateTaskInput!) {
    createTask(input: $input) {
      id
    }
  }
`);

const UpdateTaskMutation = graphql(`
  mutation UpdateTask($id: ID!, $input: UpdateTaskInput!) {
    updateTask(id: $id, input: $input) {
      id
    }
  }
`);

const TransitionTaskMutation = graphql(`
  mutation TransitionTask($id: ID!, $status: TaskStatus!) {
    transitionTask(id: $id, status: $status) {
      id
    }
  }
`);

const DeleteTaskMutation = graphql(`
  mutation DeleteTask($id: ID!) {
    deleteTask(id: $id)
  }
`);

export const tasksQuery = (variables: TasksQueryVariables) =>
  graphqlQueryOptions(TasksQuery, variables);

// Prefix of every task list query, whatever its filter and page. Writes invalidate it.
export const tasksKey = graphqlKey(TasksQuery);

export const allTasksQuery = (variables: AllTasksQueryVariables) =>
  graphqlQueryOptions(AllTasksQuery, variables);

export const createTask = graphqlMutationOptions(CreateTaskMutation);
export const updateTask = graphqlMutationOptions(UpdateTaskMutation);
export const transitionTask = graphqlMutationOptions(TransitionTaskMutation);
export const deleteTask = graphqlMutationOptions(DeleteTaskMutation);
