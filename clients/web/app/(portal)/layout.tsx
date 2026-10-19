import { Shell } from "@/components/shell";

export default function PortalLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <Shell>{children}</Shell>;
}
