import {
  AlertCircleIcon, BanknoteIcon, BrainCircuitIcon, Building2Icon, ChartNoAxesCombinedIcon,
  CheckCircle2Icon, ClockIcon, CoinsIcon, CreditCardIcon, HeadsetIcon, MailCheckIcon,
  SendIcon, UsersIcon, WalletIcon, FileChartColumnIcon, type LucideIcon,
} from "lucide-react";

const icons: Record<string, LucideIcon> = {
  "Total Tenderpreneurs": Building2Icon,
  "Active Subscriptions": CreditCardIcon,
  "Outstanding Payments": WalletIcon,
  Outstanding: WalletIcon,
  "Open Support Tickets": HeadsetIcon,
  "Total Invoiced": BanknoteIcon,
  "Total Revenue": BanknoteIcon,
  "Paid Invoices": CheckCircle2Icon,
  "AI Usage": BrainCircuitIcon,
  "Notifications Sent": SendIcon,
  "Delivered Notifications": MailCheckIcon,
  "Pending Approvals": ClockIcon,
  "Active Users": UsersIcon,
  "Reports Generated": FileChartColumnIcon,
  Paid: CheckCircle2Icon,
  "Failed Payments": AlertCircleIcon,
  "Total Usage": BrainCircuitIcon,
  "Estimated Cost": CoinsIcon,
  "Active Organisations": Building2Icon,
  Sent: SendIcon,
  Delivered: MailCheckIcon,
  Failed: AlertCircleIcon,
  Pending: ClockIcon,
  "Open Tickets": HeadsetIcon,
  "Overdue Tickets": AlertCircleIcon,
};

export function PlatformMetricCard({ label, value, trend }: { label: string; value?: string | number | null; trend?: string }) {
  const Icon = icons[label] ?? ChartNoAxesCombinedIcon;
  const available = value !== undefined && value !== null;
  return <div className="rounded-lg border bg-card p-4">
    <span className="grid size-9 place-items-center rounded-md bg-primary/10 text-primary"><Icon aria-hidden="true" className="size-4" /></span>
    <p className="mt-3 text-sm text-muted-foreground">{label}</p>
    <p className="mt-1 text-xl font-semibold tabular-nums" aria-label={available ? undefined : `${label}: not available`}>{available ? value : "—"}</p>
    {available && trend ? <p className="mt-1 text-xs text-muted-foreground">{trend}</p> : null}
  </div>;
}
