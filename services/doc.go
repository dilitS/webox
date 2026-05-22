// Package services hosts the non-provider integrations Webox needs
// outside of SSH: the GitHub REST/GraphQL client, HTTP probes for
// health checks, and similar third-party glue.
//
// The GitHub client never logs raw tokens, attaches User-Agent
// "webox/<version>", retries with jittered backoff on 5xx and rate
// limits, and returns sentinel errors for known failure modes. See
// docs/DESIGN.md §13 for the GitHub integration contract.
package services
