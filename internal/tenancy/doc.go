// Package tenancy holds the multi-tenant model: clients (tenants),
// customers, API keys, and the tenant-scoped authorization helpers that
// enforce client_id isolation inside the engine.
//
// Implementation lands in P1 (MPE-27 through MPE-31).
package tenancy
