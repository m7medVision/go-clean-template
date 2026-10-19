import { $api } from "#/api/client";
import { graphqlKey, graphqlQueryOptions } from "#/api/graphql";
import { graphql } from "#/graphql";
import type { ExchangeHistoryQueryVariables } from "#/graphql/graphql";

// The history table reads through GraphQL and asks only for the columns it shows; converting is a
// form, so it posts to REST.

const ExchangeHistoryQuery = graphql(`
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
`);

export const exchangeHistoryQuery = (variables: ExchangeHistoryQueryVariables) =>
  graphqlQueryOptions(ExchangeHistoryQuery, variables);

// Prefix of every history page query. A conversion invalidates it.
export const exchangeHistoryKey = graphqlKey(ExchangeHistoryQuery);

export const useConvert = () => $api.useMutation("post", "/v1/exchange/convert");
