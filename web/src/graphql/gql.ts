/* eslint-disable */
import * as types from './graphql';



/**
 * Map of all GraphQL operations in the project.
 *
 * This map has several performance disadvantages:
 * 1. It is not tree-shakeable, so it will include all operations in the project.
 * 2. It is not minifiable, so the string of a GraphQL query will be multiple times inside the bundle.
 * 3. It does not support dead code elimination, so it will add unused operations.
 *
 * Therefore it is highly recommended to use the babel or swc plugin for production.
 * Learn more about it here: https://the-guild.dev/graphql/codegen/plugins/presets/preset-client#reducing-bundle-size
 */
type Documents = {
    "\n  query Tasks($status: TaskStatus, $limit: Int!, $offset: Int!) {\n    tasks(status: $status, limit: $limit, offset: $offset) {\n      total\n      tasks {\n        id\n        title\n        description\n        status\n        createdAt\n        updatedAt\n      }\n    }\n  }\n": typeof types.TasksDocument,
    "\n  mutation CreateTask($input: CreateTaskInput!) {\n    createTask(input: $input) {\n      id\n    }\n  }\n": typeof types.CreateTaskDocument,
    "\n  mutation UpdateTask($id: ID!, $input: UpdateTaskInput!) {\n    updateTask(id: $id, input: $input) {\n      id\n    }\n  }\n": typeof types.UpdateTaskDocument,
    "\n  mutation TransitionTask($id: ID!, $status: TaskStatus!) {\n    transitionTask(id: $id, status: $status) {\n      id\n    }\n  }\n": typeof types.TransitionTaskDocument,
    "\n  mutation DeleteTask($id: ID!) {\n    deleteTask(id: $id)\n  }\n": typeof types.DeleteTaskDocument,
};
const documents: Documents = {
    "\n  query Tasks($status: TaskStatus, $limit: Int!, $offset: Int!) {\n    tasks(status: $status, limit: $limit, offset: $offset) {\n      total\n      tasks {\n        id\n        title\n        description\n        status\n        createdAt\n        updatedAt\n      }\n    }\n  }\n": types.TasksDocument,
    "\n  mutation CreateTask($input: CreateTaskInput!) {\n    createTask(input: $input) {\n      id\n    }\n  }\n": types.CreateTaskDocument,
    "\n  mutation UpdateTask($id: ID!, $input: UpdateTaskInput!) {\n    updateTask(id: $id, input: $input) {\n      id\n    }\n  }\n": types.UpdateTaskDocument,
    "\n  mutation TransitionTask($id: ID!, $status: TaskStatus!) {\n    transitionTask(id: $id, status: $status) {\n      id\n    }\n  }\n": types.TransitionTaskDocument,
    "\n  mutation DeleteTask($id: ID!) {\n    deleteTask(id: $id)\n  }\n": types.DeleteTaskDocument,
};

/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  query Tasks($status: TaskStatus, $limit: Int!, $offset: Int!) {\n    tasks(status: $status, limit: $limit, offset: $offset) {\n      total\n      tasks {\n        id\n        title\n        description\n        status\n        createdAt\n        updatedAt\n      }\n    }\n  }\n"): typeof import('./graphql').TasksDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation CreateTask($input: CreateTaskInput!) {\n    createTask(input: $input) {\n      id\n    }\n  }\n"): typeof import('./graphql').CreateTaskDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation UpdateTask($id: ID!, $input: UpdateTaskInput!) {\n    updateTask(id: $id, input: $input) {\n      id\n    }\n  }\n"): typeof import('./graphql').UpdateTaskDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation TransitionTask($id: ID!, $status: TaskStatus!) {\n    transitionTask(id: $id, status: $status) {\n      id\n    }\n  }\n"): typeof import('./graphql').TransitionTaskDocument;
/**
 * The graphql function is used to parse GraphQL queries into a document that can be used by GraphQL clients.
 */
export function graphql(source: "\n  mutation DeleteTask($id: ID!) {\n    deleteTask(id: $id)\n  }\n"): typeof import('./graphql').DeleteTaskDocument;


export function graphql(source: string) {
  return (documents as any)[source] ?? {};
}
