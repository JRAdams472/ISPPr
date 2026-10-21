// Shared e2e constants — the seeded demo tenant from seed/001_demo.sql.

export const API_URL = process.env.E2E_API_URL ?? "http://localhost:8090";

export const ADMIN = {
  email: "admin@isppr.local",
  password: "isppr-admin",
};

export const TENANT_KEY = "isppr_dev_demo";
export const DEMO_CLIENT = { id: "cli_demo", name: "Demo Corp" };
export const DEMO_CUSTOMER = { id: "cust_demo", name: "Demo Customer" };
export const DEMO_MASTER = { id: "tm_demo_invoice", name: "tmpl_invoice" };
