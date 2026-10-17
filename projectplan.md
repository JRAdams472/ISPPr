Document Generation Engine (DGE) Specification

1. System Architecture Overview

The system is built as a stateless, decoupled architecture utilizing a RESTful API layer, a database for client/template management, a templating middleware engine (Handlebars), and a containerized rendering pool (Gotenberg clusters).

[Client App] ---> [API Gateway (API Key / Auth)] ---> [DGE Core Engine]
                                                             |
                                            +----------------+----------------+

                                            |                |                |
                                        [DB State]   [Handlebars Engine] [Gotenberg Pool]
                                     (Clients/Specs)   (JSON Merging)     (HTML -> PDF)


2. Data Model & Multi-Tenancy

To securely separate multi-tenant customer template variants, data is partitioned by Client and Customer.

Client (Tenant): The primary organizational entity holding API credentials (e.g., a corporate SaaS partner).

Customer: A secondary entity belonging to a Client. Each Customer can possess an inherited "Master Template" override.

Template Master: The foundational HTML/CSS layout.

Template Variant: A modified version of a Master Template containing targeted CSS style modifications or conditional layout shifts specifically saved for a given Customer ID.

3. Security & Gated API Access

Authentication: Access is strictly gated via HMAC tokens or secure bearer API Keys (X-API-Key) passed in the request header.

Isolation: The API Gateway validates that the Client token owns the specific customer_id and template resources requested in the payload. Access is denied globally if keys do not match.

4. Document Request Interface (Ingress Payload)

All requests must arrive as a single POST payload containing a structural package configuration.

Endpoint: POST /api/v1/packages/generate

json

{
  "client_id": "cli_9921c3b1",
  "customer_id": "cust_448102",
  "package_name": "Q4_Onboarding_Pack",
  "documents": [
    {
      "template_id": "tmpl_invoice_master",
      "payload": {
        "invoiceNumber": "INV-2026-8941",
        "date": "October 17, 2026",
        "highPriority": true,
        "items": [
          { "description": "Cloud Infrastructure", "quantity": 1, "price": 1200.00 }
        ],
        "totalAmount": 1200.00
      }
    },
    {
      "template_id": "tmpl_welcome_letter",
      "payload": {
        "welcomeMessage": "Welcome to our premium service tier.",
        "managerName": "Alex Rivera"
      }
    }
  ]
}


Use code with caution.

5. Processing Pipeline & Execution

When a request hits the core engine, it executes the following steps sequentially:

[Payload Ingress] 
       │
       ▼
[Fetch Customer-Specific HTML/CSS Templates]
       │
       ▼
[Execute Handlebars.js Data-Binding Loops & Conditionals]
       │
       ▼
[Stream Compiled HTML Strings to Gotenberg Worker Pool]
       │
       ▼
[If Multi-Doc: Compress Binary PDFs into a Single .ZIP Archive]
       │
       ▼
[Upload Binary Store to Cloud S3 bucket with a 15-Minute TTL]
       │
       ▼
[Return Metadata JSON Response to Client]


6. Synchronous Response Interface (Egress Payload)

The API must return an immediate performance and metadata summary report. Binary files are strictly barred from this response stream.

Response (201 Created):

json

{
  "request_id": "guid_b3f982c1-d41a-4299-813c-7c01a91bc90d",
  "status": "COMPLETED",
  "created_at": "2026-10-17T06:36:00Z",
  "metrics": {
    "total_documents": 2,
    "parsing_duration_ms": 14,
    "rendering_duration_ms": 142,
    "compression_duration_ms": 22,
    "total_duration_ms": 178
  },
  "results": {
    "output_type": "ZIP",
    "filename": "Q4_Onboarding_Pack_guid_b3f982c1.zip",
    "download_url": "https://docengine.internal"
  }
}


Use code with caution.

7. Administrative & Internal Management Tools

An internal Portal UI is required to support admins, account managers, and template developers:

Client Management Dashboard: Revoke/rotate API keys, track generation volumes, view operational latencies, and debug failed requests.

Customer & Template Override UI:

A list view mapping Clients to Customers.

An editor module that pulls a Customer's template variants.

Allows internal developers to safely override default CSS styles, configure toggle blocks, or swap branding paths for a single customer without modifying core developer systems.

Evaluation of the spec

Strengths

Stateless engine + external render pool (Gotenberg) is the right shape — HTML→PDF via headless Chromium is far more maintainable than XSL-FO, scales horizontally, and isolates heavy rendering.

Handlebars (or similar logic-lite templating) for data binding keeps templates designer-editable.

Synchronous metadata response with S3 presigned-URL downloads keeps the API thin.

Multi-tenant Client/Customer/Template-variant model is reasonable and matches typical SaaS document platforms.

Gaps / risks to address

Sync rendering on the request path. Gotenberg renders typically take 300ms–2s per document. The spec's 178ms budget is optimistic, and a multi-doc package rendered serially will blow the timeout. Either render documents concurrently or add an async mode (202 Accepted + webhook/polling) for large packages.

15-minute TTL implies S3 lifecycle/presigned expiry — fine, but say "presigned URL" explicitly; also define failure/retention for debugging.

Auth: HMAC-signed requests or API keys — pick one for v1 (API keys are simpler; HMAC adds replay protection). Also specify rate limiting and per-tenant authorization checks in the engine, not just the gateway.

Template variant resolution order (customer override → client master → base template) needs an explicit merge/fallback spec.

ZIP compression — fine, but note Gotenberg can return PDF/A or merged PDFs; consider "single merged PDF" as an alternative output type.

Missing: idempotency keys (retried POSTs shouldn't double-bill/double-store), request size limits, error contract (RFC 7807), and observability (tracing across engine → Gotenberg hops, which the metrics block implies you want anyway).

Recommended stack

Given this repo is already a Go + Echo + PostgreSQL (sqlc) monolith with a Next.js frontend , reuse that stack rather than introducing Node.js for the engine:

Concern

Recommendation

Core engine / API

Go + Echo — high-throughput stateless HTTP, fits cmd/ + internal/ modular layout already used here 0-cite-0

Templating

aymerick/raymond (Handlebars-compatible Go lib) or run Handlebars in an embedded JS runtime (goja) if exact Handlebars parity is required

Render pool

Gotenberg (as specified) — stateless Docker service, horizontal scaling, /convert/html endpoint

API gateway/auth

Caddy (already in the repo as reverse proxy ) or stay in-process with Echo middleware validating X-API-Key; a dedicated gateway (Kong/Envoy) is overkill for v1

Datastore

PostgreSQL — tenants, customers, template masters/variants, API keys, request audit rows; use sqlc as the repo already does

Object storage

S3 (or MinIO locally) with presigned URLs + lifecycle rule for TTL

ZIP

archive/zip streaming to an S3 multipart upload

Admin portal

Next.js + Tailwind (reuse clients/web patterns ); template editor via CodeMirror/Monaco with a live preview rendered through Gotenberg

Observability

OpenTelemetry → the telemetry/logging compose stacks already defined in docker-compose.telemetry.yml / docker-compose.logging.yml

Why not Node.js for the engine: Handlebars is a JS library, but raymond covers the needed subset, and keeping the engine in Go avoids a second language ecosystem, matches the team's existing codebase, and gives better concurrency primitives for fan-out to the Gotenberg pool.

The spec is solid as an architecture outline; main pre-implementation work is defining the async/failure contract, variant-merge semantics, and auth details.