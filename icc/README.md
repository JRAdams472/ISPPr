# ICC profiles

Ghostscript bundles `default_cmyk.icc` in its own resource path — the
compose default (`PRINT_ICC_PROFILE=default_cmyk.icc`) emits a real
PDF/X OutputIntent with no vendored binary.

For production gang/rip work, drop a coated profile here (e.g.
`USWebCoatedSWOP.icc`, `GRACoL2013_CRPC6.icc` — Adobe ICC profiles are
freely redistributable), rebuild, and point `PRINT_ICC_PROFILE` at the
absolute path (e.g. `/usr/share/ghostscript/icc/GRACoL2013_CRPC6.icc`).
