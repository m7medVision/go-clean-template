/* eslint-disable */
/** Internal type. DO NOT USE DIRECTLY. */
type Exact<T extends { [key: string]: unknown }> = { [K in keyof T]: T[K] };
/** Internal type. DO NOT USE DIRECTLY. */
export type Incremental<T> = T | { [P in keyof T]?: P extends ' $fragmentName' | '__typename' ? T[P] : never };
import type { DocumentTypeDecoration } from '@graphql-typed-document-node/core';
export type CreateTaskInput = {
  description?: string | null | undefined;
  title: string;
};

export type TaskStatus =
  | 'DONE'
  | 'IN_PROGRESS'
  | 'TODO';

export type UpdateTaskInput = {
  description?: string | null | undefined;
  title: string;
};

export type ExchangeHistoryQueryVariables = Exact<{
  limit: number;
  offset: number;
}>;


export type ExchangeHistoryQuery = { exchangeHistory: { total: number, conversions: Array<{ from: string, to: string, amount: string, rate: string, result: string, rateDate: string }> } };

export type TasksQueryVariables = Exact<{
  status?: TaskStatus | null | undefined;
  limit: number;
  offset: number;
}>;


export type TasksQuery = { tasks: { total: number, tasks: Array<{ id: string, title: string, description: string, status: TaskStatus, createdAt: string, updatedAt: string }> } };

export type AllTasksQueryVariables = Exact<{
  status?: TaskStatus | null | undefined;
  limit: number;
  offset: number;
}>;


export type AllTasksQuery = { allTasks: { total: number, tasks: Array<{ id: string, userId: string, title: string, status: TaskStatus, createdAt: string }> } };

export type CreateTaskMutationVariables = Exact<{
  input: CreateTaskInput;
}>;


export type CreateTaskMutation = { createTask: { id: string } };

export type UpdateTaskMutationVariables = Exact<{
  id: string | number;
  input: UpdateTaskInput;
}>;


export type UpdateTaskMutation = { updateTask: { id: string } };

export type TransitionTaskMutationVariables = Exact<{
  id: string | number;
  status: TaskStatus;
}>;


export type TransitionTaskMutation = { transitionTask: { id: string } };

export type DeleteTaskMutationVariables = Exact<{
  id: string | number;
}>;


export type DeleteTaskMutation = { deleteTask: boolean };

export class TypedDocumentString<TResult, TVariables>
  extends String
  implements DocumentTypeDecoration<TResult, TVariables>
{
  __apiType?: NonNullable<DocumentTypeDecoration<TResult, TVariables>['__apiType']>;
  private value: string;
  public __meta__?: Record<string, any> | undefined;

  constructor(value: string, __meta__?: Record<string, any> | undefined) {
    super(value);
    this.value = value;
    this.__meta__ = __meta__;
  }

  override toString(): string & DocumentTypeDecoration<TResult, TVariables> {
    return this.value;
  }
}

export const ExchangeHistoryDocument = new TypedDocumentString(`
    query ExchangeHistory($limit: Int!, $offset: Int!) {
  exchangeHistory(limit: $limit, offset: $offset) {
    total
    conversions {
      from
      to
      amount
      rate
      result
      rateDate
    }
  }
}
    `) as unknown as TypedDocumentString<ExchangeHistoryQuery, ExchangeHistoryQueryVariables>;
export const TasksDocument = new TypedDocumentString(`
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
    `) as unknown as TypedDocumentString<TasksQuery, TasksQueryVariables>;
export const AllTasksDocument = new TypedDocumentString(`
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
    `) as unknown as TypedDocumentString<AllTasksQuery, AllTasksQueryVariables>;
export const CreateTaskDocument = new TypedDocumentString(`
    mutation CreateTask($input: CreateTaskInput!) {
  createTask(input: $input) {
    id
  }
}
    `) as unknown as TypedDocumentString<CreateTaskMutation, CreateTaskMutationVariables>;
export const UpdateTaskDocument = new TypedDocumentString(`
    mutation UpdateTask($id: ID!, $input: UpdateTaskInput!) {
  updateTask(id: $id, input: $input) {
    id
  }
}
    `) as unknown as TypedDocumentString<UpdateTaskMutation, UpdateTaskMutationVariables>;
export const TransitionTaskDocument = new TypedDocumentString(`
    mutation TransitionTask($id: ID!, $status: TaskStatus!) {
  transitionTask(id: $id, status: $status) {
    id
  }
}
    `) as unknown as TypedDocumentString<TransitionTaskMutation, TransitionTaskMutationVariables>;
export const DeleteTaskDocument = new TypedDocumentString(`
    mutation DeleteTask($id: ID!) {
  deleteTask(id: $id)
}
    `) as unknown as TypedDocumentString<DeleteTaskMutation, DeleteTaskMutationVariables>;