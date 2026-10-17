// archiveFilename builds the ZIP artifact name shown in generate responses:
// "<package_name>_<request_id>.zip" (see projectplan.md egress contract).
export function archiveFilename(packageName: string, requestId: string): string {
  return `${packageName}_${requestId}.zip`;
}
