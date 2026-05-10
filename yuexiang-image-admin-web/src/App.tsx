import React, { useEffect, useState } from "react";
import {
  Activity,
  AlertCircle,
  AlertTriangle,
  Archive,
  Check,
  CheckCircle2,
  ChevronRight,
  CreditCard,
  Database,
  Download,
  Edit,
  FileText,
  Globe,
  HardDrive,
  Image as ImageIcon,
  Key,
  LayoutDashboard,
  Link as LinkIcon,
  Lock,
  LogOut,
  Menu,
  Plus,
  RefreshCw,
  Search,
  Server,
  Settings,
  ShieldAlert,
  ShieldCheck,
  Trash2,
  UploadCloud,
  Users,
  X,
  Zap,
  type LucideIcon,
} from "lucide-react";

type ThemeTag = "primary" | "success" | "warning" | "danger" | "info";
type ButtonStyle = "default" | "primary" | "success" | "danger" | "warning" | "text";
type ButtonSize = "small" | "default" | "large";
type RouteID =
  | "dashboard"
  | "users"
  | "orders"
  | "plans"
  | "hidden_plans"
  | "invite_form"
  | "invite_records"
  | "moderation"
  | "security"
  | "hotlink"
  | "storage"
  | "cdn"
  | "api"
  | "queue"
  | "settings"
  | "backup"
  | "audit";
type Row = Record<string, any>;
type Column = {
  title: string;
  dataIndex: string;
  render?: (value: any, row: Row) => React.ReactNode;
};
type MenuGroup = {
  group: string;
  items: Array<{ id: RouteID; icon: LucideIcon; label: string }>;
};
type AdminUsage = {
  storage_bytes?: number;
  bandwidth_bytes?: number;
  image_requests?: number;
  api_calls?: number;
  image_process_events?: number;
};
type AdminUser = {
  id: string;
  email: string;
  nickname: string;
  plan_slug: string;
  status: string;
  created_at: string;
  usage?: AdminUsage;
};
type AdminPlan = {
  slug: string;
  name: string;
  monthly_price_cent: number;
  yearly_price_cent: number;
  visibility: "visible" | "hidden";
  purchasable: boolean;
  invite_only: boolean;
  unlimited: boolean;
  quota: {
    storage_bytes?: number | null;
    bandwidth_bytes?: number | null;
    image_requests?: number | null;
    api_calls?: number | null;
    image_process_events?: number | null;
    single_file_bytes?: number | null;
  };
};
type AdminOrder = {
  id: string;
  user_id: string;
  plan_slug: string;
  billing_cycle: string;
  amount_cent: number;
  status: string;
  ifpay_payment_id: string;
  ifpay_sub_method?: string;
  created_at: string;
  paid_at?: string;
  failed_at?: string;
  cancelled_at?: string;
  refunded_at?: string;
  operator_note?: string;
};
type AdminAccount = {
  id: string;
  email: string;
  name: string;
  role: string;
  status?: string;
  created_at?: string;
  last_login_at?: string;
};
type AdminAuthStatus = {
  setup_required: boolean;
  admin?: AdminAccount | null;
};
type AdminBootstrapStart = {
  setup_token: string;
  email: string;
  manual_entry_key: string;
  provisioning_url: string;
  qr_code_data_url: string;
  issuer: string;
  totp_app_hint: string;
  expires_in_seconds: number;
};
type AdminAuthResult = {
  admin: AdminAccount;
  token: string;
  expires_at: string;
};
type AdminOverview = {
  users: number;
  images: number;
  storage_bytes: number;
  bandwidth_bytes: number;
  risk_events: number;
  orders: number;
  paid_orders: number;
  pending_orders: number;
  revenue_cent: number;
  active_subscriptions: number;
  frozen_images: number;
  deleted_images: number;
};
type AdminSystemConfig = Row & {
  rate_limits?: {
    default_per_minute?: number;
    image_upload_per_minute?: number;
    ifpay_checkout_per_minute?: number;
    login_per_minute?: number;
  };
};
type IFPayIntegrationConfig = {
  ifpay_base_url: string;
  ifpay_partner_app_id: string;
  ifpay_client_id: string;
  ifpay_redirect_uri: string;
  ifpay_client_secret_configured: boolean;
  ifpay_private_key_configured: boolean;
  ifpay_public_key_configured: boolean;
  ifpay_webhook_public_key_configured: boolean;
  ifpay_oauth_start: string;
  ifpay_oauth_callback: string;
  ifpay_webhook: string;
  ifpay_configured: boolean;
  ifpay_payment_signing_configured: boolean;
  ifpay_webhook_verification_configured: boolean;
  updated_at?: string;
};
type Notice = { type: ThemeTag; text: string };

const API_BASE = import.meta.env.VITE_API_BASE_URL || "/api/v1";
const API_ROOT = API_BASE.replace(/\/api\/v1$/, "") || "";
const ADMIN_SESSION_KEY = "yuexiang.admin-session";
const ADMIN_SESSION_EXPIRED_EVENT = "yuexiang.admin-session-expired";

type APIEnvelope<T> = { ok: boolean; data: T; error?: { code: string; message: string } };

const getAdminSessionToken = () => localStorage.getItem(ADMIN_SESSION_KEY) || "";
const clearAdminSession = () => {
  localStorage.removeItem(ADMIN_SESSION_KEY);
  window.dispatchEvent(new Event(ADMIN_SESSION_EXPIRED_EVENT));
};

const responseErrorMessage = async (res: Response) => {
  try {
    const payload = (await res.json()) as APIEnvelope<unknown>;
    return payload.error?.message || `请求失败 (${res.status})`;
  } catch {
    return `请求失败 (${res.status})`;
  }
};

const noticeClassName = (type: ThemeTag = "danger") =>
  type === "success"
    ? "bg-[#f0f9eb] border-[#e1f3d8] text-[#67c23a]"
    : type === "warning"
      ? "bg-[#fdf6ec] border-[#faecd8] text-[#e6a23c]"
      : "bg-[#fef0f0] border-[#fde2e2] text-[#f56c6c]";

const NoticeBar = ({ notice, className = "" }: { notice: Notice | null; className?: string }) => {
  if (!notice) return null;
  return <div className={`border px-4 py-3 rounded text-sm ${noticeClassName(notice.type)} ${className}`}>{notice.text}</div>;
};

async function adminFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers);
  const token = getAdminSessionToken();
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  if (!(options.body instanceof FormData)) {
    headers.set("Content-Type", "application/json");
  }
  const res = await fetch(`${API_BASE}${path}`, { ...options, headers });
  if (res.status === 401) {
    clearAdminSession();
    throw new Error("登录状态已过期，请重新登录");
  }
  const payload = (await res.json()) as APIEnvelope<T>;
  if (!res.ok || !payload.ok) {
    throw new Error(payload.error?.message || `请求失败 (${res.status})`);
  }
  return payload.data;
}

async function adminBinaryFetch(path: string, options: RequestInit = {}) {
  const headers = new Headers(options.headers);
  const token = getAdminSessionToken();
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  const res = await fetch(`${API_BASE}${path}`, { ...options, headers });
  if (res.status === 401) {
    clearAdminSession();
    throw new Error("登录状态已过期，请重新登录");
  }
  if (!res.ok) {
    throw new Error(await responseErrorMessage(res));
  }
  return res;
}

const formatBytes = (value?: number) => {
  if (!value) return "0 B";
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  if (value < 1024 * 1024 * 1024) return `${(value / 1024 / 1024).toFixed(1)} MB`;
  if (value < 1024 * 1024 * 1024 * 1024) return `${(value / 1024 / 1024 / 1024).toFixed(1)} GB`;
  return `${(value / 1024 / 1024 / 1024 / 1024).toFixed(1)} TB`;
};
const formatMS = (value?: number) => {
  if (!value) return "0ms";
  if (value < 1000) return `${value}ms`;
  if (value < 60 * 1000) return `${Math.round(value / 1000)}s`;
  if (value < 60 * 60 * 1000) return `${Math.round(value / 1000 / 60)}m`;
  return `${Math.round(value / 1000 / 60 / 60)}h`;
};

const bytesToGBInput = (value?: number | null) => value ? String(Math.round(value / 1024 / 1024 / 1024)) : "";
const bytesToMBInput = (value?: number | null) => value ? String(Math.round(value / 1024 / 1024)) : "";
const gbInputToBytes = (value: string) => {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? Math.round(parsed * 1024 * 1024 * 1024) : null;
};
const mbInputToBytes = (value: string) => {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? Math.round(parsed * 1024 * 1024) : null;
};
const numberInputOrNull = (value: string) => {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? Math.round(parsed) : null;
};
const downloadText = (filename: string, text: string, contentType = "text/csv;charset=utf-8") => {
  const blob = new Blob([text], { type: contentType });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
};

const THEME = {
  primary: "#409eff",
  success: "#67c23a",
  warning: "#e6a23c",
  danger: "#f56c6c",
  info: "#909399",
  textPrimary: "#303133",
  textRegular: "#606266",
  textSecondary: "#909399",
  borderBase: "#dcdfe6",
  borderLight: "#e4e7ed",
  bgBase: "#f2f6fc",
};

const injectGlobalStyles = () => {
  if (typeof document === "undefined" || document.getElementById("yuexiang-admin-theme")) {
    return;
  }
  const style = document.createElement("style");
  style.id = "yuexiang-admin-theme";
  style.innerHTML = `
    :root {
      --font-family-sans: 'HarmonyOS Sans SC', 'MiSans', 'Alibaba PuHuiTi 3.0', 'PingFang SC', 'Microsoft YaHei', sans-serif;
    }
    body {
      font-family: var(--font-family-sans);
      color: ${THEME.textRegular};
      background-color: ${THEME.bgBase};
      margin: 0;
    }
    h1, h2, h3, h4, h5, h6 { color: ${THEME.textPrimary}; font-weight: 600; }
    .el-shadow { box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1); }
    .el-shadow-light { box-shadow: 0 2px 4px rgba(0, 0, 0, .12), 0 0 6px rgba(0, 0, 0, .04); }
    ::-webkit-scrollbar { width: 8px; height: 8px; }
    ::-webkit-scrollbar-track { background: transparent; }
    ::-webkit-scrollbar-thumb { background: #c0c4cc; border-radius: 4px; }
    ::-webkit-scrollbar-thumb:hover { background: #909399; }
  `;
  document.head.appendChild(style);
};

const Card = ({
  title,
  extra,
  children,
  className = "",
}: {
  title?: React.ReactNode;
  extra?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}) => (
  <div className={`bg-white rounded-md border border-[#e4e7ed] el-shadow-light transition-all duration-300 hover:shadow-md ${className}`}>
    {(title || extra) && (
      <div className="flex justify-between items-center px-5 py-4 border-b border-[#ebeef5]">
        <div className="font-semibold text-[#303133] text-sm flex items-center gap-2">{title}</div>
        <div>{extra}</div>
      </div>
    )}
    <div className="p-5">{children}</div>
  </div>
);

const Button = ({
  children,
  type = "default",
  htmlType = "button",
  size = "default",
  icon: Icon,
  onClick,
  className = "",
  disabled = false,
}: {
  children?: React.ReactNode;
  type?: ButtonStyle;
  htmlType?: "button" | "submit";
  size?: ButtonSize;
  icon?: LucideIcon;
  onClick?: () => void;
  className?: string;
  disabled?: boolean;
}) => {
  const baseStyle = "inline-flex items-center justify-center font-medium transition-colors border rounded focus:outline-none cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed";
  const typeStyles: Record<ButtonStyle, string> = {
    default: "bg-white border-[#dcdfe6] text-[#606266] hover:text-[#409eff] hover:border-[#c6e2ff] hover:bg-[#ecf5ff]",
    primary: "bg-[#409eff] border-[#409eff] text-white hover:bg-[#66b1ff] hover:border-[#66b1ff]",
    success: "bg-[#67c23a] border-[#67c23a] text-white hover:bg-[#85ce61] hover:border-[#85ce61]",
    danger: "bg-[#f56c6c] border-[#f56c6c] text-white hover:bg-[#f78989] hover:border-[#f78989]",
    warning: "bg-[#e6a23c] border-[#e6a23c] text-white hover:bg-[#ebb563] hover:border-[#ebb563]",
    text: "bg-transparent border-transparent text-[#409eff] hover:text-[#66b1ff] p-0",
  };
  const sizeStyles: Record<ButtonSize, string> = {
    small: "px-3 py-1.5 text-xs",
    default: "px-4 py-2 text-sm",
    large: "px-5 py-2.5 text-base",
  };

  return (
    <button type={htmlType} onClick={onClick} disabled={disabled} className={`${baseStyle} ${typeStyles[type]} ${sizeStyles[size]} ${className}`}>
      {Icon && <Icon className={`w-4 h-4 ${children ? "mr-1.5" : ""}`} />}
      {children}
    </button>
  );
};

const Tag = ({ children, type = "info", className = "" }: { children: React.ReactNode; type?: ThemeTag; className?: string }) => {
  const styles: Record<ThemeTag, string> = {
    primary: "bg-[#ecf5ff] text-[#409eff] border-[#d9ecff]",
    success: "bg-[#f0f9eb] text-[#67c23a] border-[#e1f3d8]",
    warning: "bg-[#fdf6ec] text-[#e6a23c] border-[#faecd8]",
    danger: "bg-[#fef0f0] text-[#f56c6c] border-[#fde2e2]",
    info: "bg-[#f4f4f5] text-[#909399] border-[#e9e9eb]",
  };
  return <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs border ${styles[type]} ${className}`}>{children}</span>;
};

const Input = ({
  placeholder,
  value,
  onChange,
  type = "text",
  prefix: PrefixIcon,
  className = "",
  disabled = false,
}: {
  placeholder?: string;
  value?: string;
  onChange?: (value: string) => void;
  type?: string;
  prefix?: LucideIcon;
  className?: string;
  disabled?: boolean;
}) => (
  <div className={`relative ${className}`}>
    {PrefixIcon && <PrefixIcon className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[#c0c4cc]" />}
    <input
      type={type}
      placeholder={placeholder}
      value={value}
      readOnly={value !== undefined && !onChange}
      disabled={disabled}
      onChange={(e) => onChange?.(e.target.value)}
      className={`w-full bg-white border border-[#dcdfe6] text-[#606266] rounded px-3 py-2 text-sm transition-colors focus:outline-none focus:border-[#409eff] placeholder-[#c0c4cc] disabled:bg-[#f5f7fa] disabled:text-[#909399] ${PrefixIcon ? "pl-9" : ""}`}
    />
  </div>
);

const FormItem = ({
  label,
  children,
  required,
  helpText,
}: {
  label: string;
  children: React.ReactNode;
  required?: boolean;
  helpText?: string;
}) => (
  <div className="mb-5 flex flex-col md:flex-row md:items-start gap-2 md:gap-4">
    <label className={`md:w-32 text-sm text-[#606266] md:text-right pt-2 ${required ? 'before:content-["*"] before:text-[#f56c6c] before:mr-1' : ""}`}>
      {label}
    </label>
    <div className="flex-1">
      {children}
      {helpText && <div className="text-xs text-[#909399] mt-1.5 leading-relaxed">{helpText}</div>}
    </div>
  </div>
);

const Table = ({ columns, data, emptyText = "暂无数据" }: { columns: Column[]; data: Row[]; emptyText?: string }) => (
  <div className="w-full overflow-x-auto border border-[#ebeef5] rounded-sm">
    <table className="w-full text-sm text-left text-[#606266]">
      <thead className="text-[#909399] bg-[#f5f7fa] font-medium border-b border-[#ebeef5]">
        <tr>
          {columns.map((col) => (
            <th key={col.title} className="px-4 py-3 whitespace-nowrap">{col.title}</th>
          ))}
        </tr>
      </thead>
      <tbody>
        {data.map((row, rowIndex) => (
          <tr key={row.id ?? rowIndex} className="border-b border-[#ebeef5] hover:bg-[#f5f7fa] transition-colors last:border-0">
            {columns.map((col) => (
              <td key={col.title} className="px-4 py-3">
                {col.render ? col.render(row[col.dataIndex], row) : row[col.dataIndex]}
              </td>
            ))}
          </tr>
        ))}
        {data.length === 0 && (
          <tr>
            <td colSpan={columns.length} className="px-4 py-10 text-center text-[#909399] text-sm">
              {emptyText}
            </td>
          </tr>
        )}
      </tbody>
    </table>
  </div>
);

const Dialog = ({
  visible,
  title,
  onClose,
  onConfirm,
  children,
  confirmText = "确定",
  danger = false,
  confirmDisabled = false,
}: {
  visible: boolean;
  title: string;
  onClose: () => void;
  onConfirm: () => void;
  children: React.ReactNode;
  confirmText?: string;
  danger?: boolean;
  confirmDisabled?: boolean;
}) => {
  if (!visible) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
      <div className="bg-white rounded-md shadow-xl w-full max-w-lg mx-4 overflow-hidden">
        <div className="px-5 py-4 border-b border-[#ebeef5] flex justify-between items-center">
          <h3 className="text-base font-semibold text-[#303133]">{title}</h3>
          <button type="button" onClick={onClose} className="text-[#909399] hover:text-[#f56c6c] transition-colors">
            <X className="w-5 h-5" />
          </button>
        </div>
        <div className="px-5 py-6 text-sm text-[#606266]">{children}</div>
        <div className="px-5 py-3 border-t border-[#ebeef5] bg-[#fbfdff] flex justify-end gap-3">
          <Button onClick={onClose}>取消</Button>
          <Button type={danger ? "danger" : "primary"} disabled={confirmDisabled} onClick={onConfirm}>{confirmText}</Button>
        </div>
      </div>
    </div>
  );
};

const MetricCard = ({
  title,
  value,
  subValue,
  trend,
  trendValue,
  icon: Icon,
  colorClass,
}: {
  title: string;
  value: string;
  subValue?: string;
  trend?: "up" | "down";
  trendValue?: string;
  icon: LucideIcon;
  colorClass: string;
}) => (
  <Card className="relative overflow-hidden group">
    <div className={`absolute top-0 right-0 w-16 h-16 -mr-6 -mt-6 rounded-full opacity-10 transition-transform group-hover:scale-150 ${colorClass}`} />
    <div className="flex justify-between items-start mb-2">
      <span className="text-[#909399] text-sm font-medium">{title}</span>
      <Icon className={`w-5 h-5 opacity-70 ${colorClass.replace("bg-", "text-")}`} />
    </div>
    <div className="text-2xl font-bold text-[#303133] mb-1 font-mono tracking-tight">{value}</div>
    {subValue && <div className="text-xs text-[#909399] mb-2">{subValue}</div>}
    {trend && (
      <div className={`text-xs flex items-center ${trend === "up" ? "text-[#f56c6c]" : "text-[#67c23a]"}`}>
        {trend === "up" ? "↑" : "↓"} {trendValue} 较上月
      </div>
    )}
  </Card>
);

const DashboardView = () => {
  const [overview, setOverview] = useState<AdminOverview | null>(null);
  const [notice, setNotice] = useState("");
  useEffect(() => {
    adminFetch<AdminOverview>("/admin/overview")
      .then((data) => {
        setNotice("");
        setOverview(data);
      })
      .catch((error) => {
        setOverview(null);
        setNotice(error instanceof Error ? error.message : "总览读取失败，请检查 API 服务。");
      });
  }, []);
  return (
    <div className="space-y-6">
      {notice && <div className="bg-[#fef0f0] border border-[#fde2e2] text-[#f56c6c] px-4 py-3 rounded text-sm">{notice}</div>}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <MetricCard title="总用户数" value={String(overview?.users ?? 0)} subValue={overview ? "生产实时指标" : "等待 API 返回"} icon={Users} colorClass="bg-blue-500" />
        <MetricCard title="订单收入" value={`¥ ${((overview?.revenue_cent ?? 0) / 100).toFixed(2)}`} subValue={`已支付 ${overview?.paid_orders ?? 0} / 待处理 ${overview?.pending_orders ?? 0}`} icon={CreditCard} colorClass="bg-green-500" />
        <MetricCard title="存储总占用" value={overview ? formatBytes(overview.storage_bytes) : "0 B"} subValue={`对象 ${overview?.images ?? 0} / 冻结 ${overview?.frozen_images ?? 0}`} icon={Database} colorClass="bg-purple-500" />
        <MetricCard title="风险拦截事件" value={String(overview?.risk_events ?? 0)} subValue="WAF & 防盗链" icon={ShieldAlert} colorClass="bg-red-500" />
      </div>

    <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <div className="lg:col-span-2">
        <Card title="资源消耗概览">
          <div className="flex gap-4 mb-2">
            <div className="text-sm"><span className="inline-block w-3 h-3 bg-[#409eff] rounded-sm mr-2" />对象存储</div>
            <div className="text-sm"><span className="inline-block w-3 h-3 bg-[#e6a23c] rounded-sm mr-2" />累计分发</div>
          </div>
          <div className="grid md:grid-cols-3 gap-4 mt-4">
            <div className="border border-[#ebeef5] rounded p-4 bg-[#f5f7fa]">
              <div className="text-xs text-[#909399] mb-1">存储容量</div>
              <div className="text-2xl font-bold font-mono text-[#303133]">{formatBytes(overview?.storage_bytes)}</div>
            </div>
            <div className="border border-[#ebeef5] rounded p-4 bg-[#f5f7fa]">
              <div className="text-xs text-[#909399] mb-1">分发流量</div>
              <div className="text-2xl font-bold font-mono text-[#303133]">{formatBytes(overview?.bandwidth_bytes)}</div>
            </div>
            <div className="border border-[#ebeef5] rounded p-4 bg-[#f5f7fa]">
              <div className="text-xs text-[#909399] mb-1">活跃订阅</div>
              <div className="text-2xl font-bold font-mono text-[#303133]">{overview?.active_subscriptions ?? 0}</div>
            </div>
          </div>
        </Card>
      </div>
      <Card title="系统健康与安全态势">
        <div className="space-y-4">
          <div className="flex justify-between items-center p-3 bg-[#f0f9eb] rounded border border-[#e1f3d8]">
            <div className="flex items-center gap-2 text-[#67c23a]"><CheckCircle2 className="w-5 h-5" /> 后端 API</div>
            <span className="text-xs font-bold">{overview ? "已连接" : "检查中"}</span>
          </div>
          <div className="flex justify-between items-center p-3 bg-[#f0f9eb] rounded border border-[#e1f3d8]">
            <div className="flex items-center gap-2 text-[#67c23a]"><Server className="w-5 h-5" /> 订单系统</div>
            <span className="text-xs font-bold">{overview?.orders ?? 0} 单</span>
          </div>
          <div className="flex justify-between items-center p-3 bg-[#fdf6ec] rounded border border-[#faecd8]">
            <div className="flex items-center gap-2 text-[#e6a23c]"><Zap className="w-5 h-5" /> 删除对象</div>
            <span className="text-xs">{overview?.deleted_images ?? 0} 个</span>
          </div>
          <div className="flex justify-between items-center p-3 bg-[#fef0f0] rounded border border-[#fde2e2]">
            <div className="flex items-center gap-2 text-[#f56c6c]"><ShieldAlert className="w-5 h-5" /> 恶意爬虫告警</div>
            <span className="text-xs font-bold">{overview?.risk_events ?? 0} 事件</span>
          </div>
        </div>
      </Card>
    </div>
    </div>
  );
};

const UserManageView = () => {
  const [users, setUsers] = useState<Row[]>([]);
  const [keyword, setKeyword] = useState("");
  const [notice, setNotice] = useState<{ type: ThemeTag; text: string } | null>(null);
  const [expiringUser, setExpiringUser] = useState<Row | null>(null);
  const [selectedUser, setSelectedUser] = useState<Row | null>(null);
  const [expireReason, setExpireReason] = useState("");
  const [userActionID, setUserActionID] = useState("");
  const [expiring, setExpiring] = useState(false);
  const loadUsers = () => {
    adminFetch<AdminUser[]>("/admin/users")
      .then((rows) => setUsers(rows.map((user) => ({
        id: user.id,
        username: user.nickname || user.email.split("@")[0],
        email: user.email,
        plan: user.plan_slug || "未订阅",
        storageUsed: formatBytes(user.usage?.storage_bytes),
        bandwidthUsed: formatBytes(user.usage?.bandwidth_bytes),
        imageRequests: user.usage?.image_requests || 0,
        apiCalls: user.usage?.api_calls || 0,
        processEvents: user.usage?.image_process_events || 0,
        status: user.status,
        createdAt: user.created_at?.replace("T", " ").slice(0, 19) || "-",
        riskLevel: user.status === "banned" ? "high" : (user.usage?.bandwidth_bytes || 0) > 1024 * 1024 * 1024 * 1024 ? "medium" : "low",
      }))))
      .catch((error) => {
        setUsers([]);
        setNotice({ type: "danger", text: error instanceof Error ? error.message : "用户列表读取失败，请检查 API 服务。" });
      });
  };
  useEffect(loadUsers, []);
  const filteredUsers = users.filter((user) => {
    const query = keyword.trim().toLowerCase();
    if (!query) return true;
    return [user.id, user.username, user.email, user.plan, user.status].some((value) => String(value).toLowerCase().includes(query));
  });
  const exportUsers = () => {
    const rows = [["id", "username", "email", "plan", "status", "storage_used", "created_at"], ...filteredUsers.map((user) => [
      user.id,
      user.username,
      user.email,
      user.plan,
      user.status,
      user.storageUsed,
      user.createdAt,
    ])];
    downloadText("yuexiang-users.csv", rows.map((row) => row.map((cell) => `"${String(cell).replaceAll("\"", "\"\"")}"`).join(",")).join("\n"));
  };
  const setUserStatus = async (userID: string, action: "ban" | "unban") => {
    if (userActionID) return;
    setUserActionID(userID);
    try {
      await adminFetch(`/admin/users/${encodeURIComponent(userID)}/${action}`, {
        method: "POST",
        body: JSON.stringify({ reason: action === "ban" ? "管理员后台封禁" : "管理员后台解封" }),
      });
      setNotice({ type: "success", text: action === "ban" ? "账号已封禁，API Key 和会话访问将被拒绝。" : "账号已恢复为正常状态。" });
      loadUsers();
    } catch (error) {
      setNotice({ type: "danger", text: error instanceof Error ? error.message : "操作失败，请检查后台服务。" });
    } finally {
      setUserActionID("");
    }
  };
  const expireSubscription = async () => {
    if (!expiringUser || expiring) return;
    if (!expireReason.trim()) {
      setNotice({ type: "danger", text: "请填写强制到期的审计原因。" });
      return;
    }
    setExpiring(true);
    try {
      await adminFetch(`/admin/users/${encodeURIComponent(String(expiringUser.id))}/subscription/expire`, {
        method: "POST",
        body: JSON.stringify({ reason: expireReason.trim(), retention_days: 30 }),
      });
      setNotice({ type: "success", text: "订阅已切入 30 天只读保留期，用户将无法继续上传。" });
      setExpiringUser(null);
      setExpireReason("");
      loadUsers();
    } catch (error) {
      setNotice({ type: "danger", text: error instanceof Error ? error.message : "订阅到期操作失败，请检查后台服务。" });
    } finally {
      setExpiring(false);
    }
  };
  return (
    <Card title="用户管理">
      {notice && <div className={`mb-4 border px-4 py-3 rounded text-sm ${notice.type === "success" ? "bg-[#f0f9eb] border-[#e1f3d8] text-[#67c23a]" : "bg-[#fef0f0] border-[#fde2e2] text-[#f56c6c]"}`}>{notice.text}</div>}
      <div className="flex gap-4 mb-4">
        <Input value={keyword} onChange={setKeyword} placeholder="搜索用户名/邮箱/ID" prefix={Search} className="w-64" />
        <Button icon={Search} type="primary" onClick={() => setKeyword(keyword.trim())}>搜索</Button>
        <div className="flex-1" />
        <Button icon={Download} onClick={exportUsers}>导出数据</Button>
      </div>
      <Table
      columns={[
        { title: "用户 ID", dataIndex: "id", render: (val) => <span className="font-mono text-xs text-[#909399]">{val}</span> },
        { title: "账号信息", dataIndex: "username", render: (val, row) => <div><div className="font-medium text-[#409eff]">{val}</div><div className="text-xs text-[#909399]">{row.email}</div></div> },
        { title: "当前套餐", dataIndex: "plan", render: (val) => <Tag type={val === "Infinite Max" ? "warning" : "primary"}>{val}</Tag> },
        { title: "存储用量", dataIndex: "storageUsed" },
        { title: "风控评级", dataIndex: "riskLevel", render: (val) => {
          const map: Record<string, { t: string; c: ThemeTag }> = { low: { t: "安全", c: "success" }, medium: { t: "关注", c: "warning" }, high: { t: "高危", c: "danger" } };
          return <Tag type={map[val].c}>{map[val].t}</Tag>;
        } },
        { title: "状态", dataIndex: "status", render: (val) => val === "active" ? <span className="text-[#67c23a] flex items-center gap-1"><CheckCircle2 className="w-3 h-3" /> 正常</span> : <span className="text-[#f56c6c] flex items-center gap-1"><Lock className="w-3 h-3" /> 封禁</span> },
        { title: "注册时间", dataIndex: "createdAt", render: (val) => <span className="text-xs">{val}</span> },
        { title: "操作", dataIndex: "id", render: (_, row) => (
          <div className="flex gap-2">
            <Button type="text" size="small" onClick={() => setSelectedUser(row)}>详情</Button>
            {row.status === "active" ? (
              <Button type="text" size="small" disabled={Boolean(userActionID)} className="text-[#f56c6c] hover:text-[#f78989]" onClick={() => void setUserStatus(String(row.id), "ban")}>{userActionID === row.id ? "处理中" : "封禁"}</Button>
            ) : (
              <Button type="text" size="small" disabled={Boolean(userActionID)} className="text-[#67c23a] hover:text-[#85ce61]" onClick={() => void setUserStatus(String(row.id), "unban")}>{userActionID === row.id ? "处理中" : "解封"}</Button>
            )}
            {row.plan !== "未订阅" && <Button type="text" size="small" disabled={Boolean(userActionID) || expiring} className="text-[#e6a23c] hover:text-[#ebb563]" onClick={() => { setExpiringUser(row); setExpireReason(`运营手动结束订阅：${row.email}`); }}>到期</Button>}
          </div>
        ) },
      ]}
        data={filteredUsers}
        emptyText="没有匹配的用户。"
      />
      <Dialog
        visible={Boolean(expiringUser)}
        title="强制订阅到期"
        danger
        confirmText={expiring ? "处理中..." : "确认进入保留期"}
        confirmDisabled={expiring}
        onClose={() => { if (!expiring) { setExpiringUser(null); setExpireReason(""); } }}
        onConfirm={() => void expireSubscription()}
      >
        <div className="space-y-4">
          <p>用户 <strong>{String(expiringUser?.email || "")}</strong> 的当前订阅会立即停止写入，并进入 30 天只读保留期。</p>
          <FormItem label="操作原因" required>
            <Input value={expireReason} onChange={setExpireReason} placeholder="请输入审计原因" />
          </FormItem>
        </div>
      </Dialog>
      <Dialog
        visible={Boolean(selectedUser)}
        title="用户详情"
        confirmText="关闭"
        onClose={() => setSelectedUser(null)}
        onConfirm={() => setSelectedUser(null)}
      >
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3 text-sm">
          {[
            ["用户 ID", selectedUser?.id],
            ["邮箱", selectedUser?.email],
            ["昵称", selectedUser?.username],
            ["套餐", selectedUser?.plan],
            ["状态", selectedUser?.status],
            ["注册时间", selectedUser?.createdAt],
            ["存储用量", selectedUser?.storageUsed],
            ["分发流量", selectedUser?.bandwidthUsed],
            ["图片请求", selectedUser?.imageRequests],
            ["API 调用", selectedUser?.apiCalls],
            ["边缘处理", selectedUser?.processEvents],
          ].map(([label, value]) => (
            <div key={String(label)} className="border border-[#ebeef5] rounded p-3 bg-[#f5f7fa]">
              <div className="text-xs text-[#909399] mb-1">{label}</div>
              <div className="font-mono text-[#303133] break-all">{String(value ?? "-")}</div>
            </div>
          ))}
        </div>
      </Dialog>
    </Card>
  );
};

const PlanManageView = () => {
  const emptyPlanForm = {
    slug: "",
    name: "",
    monthly: "29",
    yearly: "299",
    storageGB: "50",
    trafficGB: "500",
    requests: "10000000",
    apiCalls: "500000",
    processEvents: "20000",
    singleFileMB: "50",
    visible: true,
    purchasable: true,
  };
  const [plans, setPlans] = useState<AdminPlan[]>([]);
  const [form, setForm] = useState(emptyPlanForm);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);
  const [savingPlan, setSavingPlan] = useState(false);
  const [unpublishingSlug, setUnpublishingSlug] = useState("");
  const [showHiddenPlans, setShowHiddenPlans] = useState(false);
  const loadPlans = () => {
    adminFetch<AdminPlan[]>("/admin/plans")
      .then(setPlans)
      .catch((error) => {
        setPlans([]);
        setNotice({ type: "danger", text: error instanceof Error ? error.message : "套餐列表读取失败，请检查 API 服务。" });
      });
  };
  useEffect(loadPlans, []);
  const openPlanDialog = (plan?: AdminPlan) => {
    if (savingPlan) return;
    if (!plan) {
      setForm(emptyPlanForm);
    } else {
      setForm({
        slug: plan.slug,
        name: plan.name,
        monthly: String(Math.round(plan.monthly_price_cent / 100)),
        yearly: String(Math.round(plan.yearly_price_cent / 100)),
        storageGB: bytesToGBInput(plan.quota?.storage_bytes),
        trafficGB: bytesToGBInput(plan.quota?.bandwidth_bytes),
        requests: plan.quota?.image_requests ? String(plan.quota.image_requests) : "",
        apiCalls: plan.quota?.api_calls ? String(plan.quota.api_calls) : "",
        processEvents: plan.quota?.image_process_events ? String(plan.quota.image_process_events) : "",
        singleFileMB: bytesToMBInput(plan.quota?.single_file_bytes),
        visible: plan.visibility !== "hidden",
        purchasable: plan.purchasable,
      });
    }
    setDialogOpen(true);
  };
  const buildPlanPayload = (override?: Partial<AdminPlan>): AdminPlan => {
    const hidden = !form.visible;
    return {
      slug: form.slug.trim().toLowerCase(),
      name: form.name.trim(),
      monthly_price_cent: Math.round((Number(form.monthly) || 0) * 100),
      yearly_price_cent: Math.round((Number(form.yearly) || 0) * 100),
      visibility: hidden ? "hidden" : "visible",
      purchasable: hidden ? false : form.purchasable,
      invite_only: hidden,
      unlimited: false,
      quota: {
        storage_bytes: gbInputToBytes(form.storageGB),
        bandwidth_bytes: gbInputToBytes(form.trafficGB),
        image_requests: numberInputOrNull(form.requests),
        api_calls: numberInputOrNull(form.apiCalls),
        image_process_events: numberInputOrNull(form.processEvents),
        single_file_bytes: mbInputToBytes(form.singleFileMB),
      },
      ...override,
    };
  };
  const savePlan = async () => {
    if (savingPlan) return;
    setSavingPlan(true);
    try {
      const payload = buildPlanPayload();
      await adminFetch<AdminPlan>("/admin/plans", { method: "POST", body: JSON.stringify(payload) });
      setNotice({ type: "success", text: `套餐 ${payload.name} 已保存。` });
      setDialogOpen(false);
      loadPlans();
    } catch (error) {
      setNotice({ type: "danger", text: error instanceof Error ? error.message : "套餐保存失败，请检查后台服务。" });
    } finally {
      setSavingPlan(false);
    }
  };
  const setPlanPublication = async (plan: AdminPlan, publish: boolean) => {
    if (unpublishingSlug) return;
    setUnpublishingSlug(plan.slug);
    try {
      await adminFetch<AdminPlan>("/admin/plans", {
        method: "POST",
        body: JSON.stringify({ ...plan, visibility: publish ? "visible" : "hidden", purchasable: publish, invite_only: !publish }),
      });
      setNotice({ type: "success", text: publish ? `${plan.name} 已重新上架。` : `${plan.name} 已下架，前台不再展示或购买。` });
      loadPlans();
    } catch (error) {
      setNotice({ type: "danger", text: error instanceof Error ? error.message : "套餐上下架失败，请检查后台服务。" });
    } finally {
      setUnpublishingSlug("");
    }
  };
  const visiblePlans = plans.filter((plan) => plan.slug !== "infinite-max" && (showHiddenPlans || plan.visibility !== "hidden"));
  return (
    <div className="space-y-4">
      <NoticeBar notice={notice} />
      <div className="flex justify-between items-center">
        <h2 className="text-lg font-semibold text-[#303133]">公开套餐配置</h2>
        <div className="flex items-center gap-2">
          <Button icon={showHiddenPlans ? CheckCircle2 : AlertCircle} onClick={() => setShowHiddenPlans(!showHiddenPlans)}>
            {showHiddenPlans ? "只看上架" : "显示已下架"}
          </Button>
          <Button type="primary" icon={Plus} disabled={savingPlan || Boolean(unpublishingSlug)} onClick={() => openPlanDialog()}>新增套餐</Button>
        </div>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {visiblePlans.map((plan) => (
        <Card key={plan.slug} className="relative">
          <div className="flex justify-between items-start mb-4 border-b border-[#ebeef5] pb-3">
            <div>
              <div className="text-xl font-bold text-[#303133]">{plan.name}</div>
              <div className="text-[#f56c6c] font-semibold mt-1">¥{Math.round(plan.monthly_price_cent / 100)}<span className="text-sm text-[#909399] font-normal">/月</span></div>
            </div>
            <Tag type={plan.visibility === "visible" && plan.purchasable ? "success" : "info"}>{plan.visibility === "visible" && plan.purchasable ? "上架中" : "已下架"}</Tag>
          </div>
          <div className="space-y-2 text-sm text-[#606266] mb-6">
            <div className="flex justify-between"><span>存储空间</span><span className="font-mono">{plan.quota?.storage_bytes ? formatBytes(plan.quota.storage_bytes) : "不限量"}</span></div>
            <div className="flex justify-between"><span>月访问流量</span><span className="font-mono">{plan.quota?.bandwidth_bytes ? formatBytes(plan.quota.bandwidth_bytes) : "不限量"}</span></div>
            <div className="flex justify-between"><span>图片请求数</span><span className="font-mono">{plan.quota?.image_requests ? `${plan.quota.image_requests.toLocaleString()}次` : "不限量"}</span></div>
            <div className="flex justify-between"><span>API 调用</span><span className="font-mono">{plan.quota?.api_calls ? `${plan.quota.api_calls.toLocaleString()}次` : "不限量"}</span></div>
          </div>
          <div className="flex gap-2">
            <Button className="flex-1" icon={Edit} disabled={savingPlan || Boolean(unpublishingSlug)} onClick={() => openPlanDialog(plan)}>编辑</Button>
            {plan.visibility === "hidden" || !plan.purchasable ? (
              <Button className="flex-1" type="success" icon={CheckCircle2} disabled={Boolean(unpublishingSlug)} onClick={() => void setPlanPublication(plan, true)}>
                {unpublishingSlug === plan.slug ? "上架中..." : "上架"}
              </Button>
            ) : (
              <Button className="flex-1" type="danger" icon={Trash2} disabled={plan.slug === "go" || Boolean(unpublishingSlug)} onClick={() => void setPlanPublication(plan, false)}>
                {unpublishingSlug === plan.slug ? "下架中..." : "下架"}
              </Button>
            )}
          </div>
        </Card>
        ))}
        {visiblePlans.length === 0 && <div className="text-sm text-[#909399] p-8 bg-white border border-[#ebeef5] rounded">暂无套餐配置。</div>}
      </div>
      <Dialog
        visible={dialogOpen}
        title="套餐配置"
        onClose={() => { if (!savingPlan) setDialogOpen(false); }}
        onConfirm={() => void savePlan()}
        confirmText={savingPlan ? "保存中..." : "保存套餐"}
        confirmDisabled={savingPlan || !form.slug.trim() || !form.name.trim()}
      >
        <div className="grid grid-cols-1 md:grid-cols-2 gap-x-4">
          <FormItem label="Slug" required><Input value={form.slug} onChange={(value) => setForm({ ...form, slug: value })} placeholder="plus-enterprise" /></FormItem>
          <FormItem label="套餐名称" required><Input value={form.name} onChange={(value) => setForm({ ...form, name: value })} placeholder="Enterprise" /></FormItem>
          <FormItem label="月付价格"><Input type="number" value={form.monthly} onChange={(value) => setForm({ ...form, monthly: value })} /></FormItem>
          <FormItem label="年付价格"><Input type="number" value={form.yearly} onChange={(value) => setForm({ ...form, yearly: value })} /></FormItem>
          <FormItem label="存储 GB"><Input type="number" value={form.storageGB} onChange={(value) => setForm({ ...form, storageGB: value })} placeholder="留空为不限量" /></FormItem>
          <FormItem label="流量 GB"><Input type="number" value={form.trafficGB} onChange={(value) => setForm({ ...form, trafficGB: value })} placeholder="留空为不限量" /></FormItem>
          <FormItem label="HTTP 请求"><Input type="number" value={form.requests} onChange={(value) => setForm({ ...form, requests: value })} /></FormItem>
          <FormItem label="API 调用"><Input type="number" value={form.apiCalls} onChange={(value) => setForm({ ...form, apiCalls: value })} /></FormItem>
          <FormItem label="边缘处理"><Input type="number" value={form.processEvents} onChange={(value) => setForm({ ...form, processEvents: value })} /></FormItem>
          <FormItem label="单文件 MB"><Input type="number" value={form.singleFileMB} onChange={(value) => setForm({ ...form, singleFileMB: value })} /></FormItem>
        </div>
        <div className="flex gap-6 pt-2 pl-2">
          <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={form.visible} onChange={(event) => setForm({ ...form, visible: event.target.checked, purchasable: event.target.checked ? form.purchasable : false })} /> 前台可见</label>
          <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={form.purchasable} disabled={!form.visible} onChange={(event) => setForm({ ...form, purchasable: event.target.checked })} /> 允许购买</label>
        </div>
      </Dialog>
    </div>
  );
};

const HiddenPlanView = () => {
  const [showConfirm, setShowConfirm] = useState(false);
  const [target, setTarget] = useState("");
  const [grantDays, setGrantDays] = useState("365");
  const [reason, setReason] = useState("");
  const [notice, setNotice] = useState<{ type: ThemeTag; text: string } | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const grantInfiniteMax = async () => {
    if (!target.trim() || !reason.trim()) {
      setNotice({ type: "danger", text: "请填写目标用户和审计原因。" });
      return;
    }
    setSubmitting(true);
    try {
      await adminFetch(`/admin/users/${encodeURIComponent(target.trim())}/grant-plan`, {
        method: "POST",
        body: JSON.stringify({
          plan_slug: "infinite-max",
          days: Number(grantDays) || 365,
          reason: reason.trim(),
        }),
      });
      setNotice({ type: "success", text: `已向 ${target.trim()} 发放 Infinite Max，并写入审计日志。` });
      setShowConfirm(false);
      setTarget("");
      setReason("");
      setGrantDays("365");
    } catch (error) {
      setNotice({ type: "danger", text: error instanceof Error ? error.message : "发放失败，请检查后台服务。" });
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="space-y-6">
      {notice && (
        <div className={`border px-4 py-3 rounded-md text-sm ${notice.type === "success" ? "bg-[#f0f9eb] border-[#e1f3d8] text-[#67c23a]" : "bg-[#fef0f0] border-[#fde2e2] text-[#f56c6c]"}`}>
          {notice.text}
        </div>
      )}

      <div className="bg-[#fdf6ec] border border-[#faecd8] p-4 rounded-md flex items-start gap-3">
        <AlertTriangle className="text-[#e6a23c] w-6 h-6 flex-shrink-0 mt-0.5" />
        <div>
          <h4 className="text-[#e6a23c] font-bold text-base mb-1">内部隐藏套餐管理 - Infinite Max</h4>
          <p className="text-sm text-[#e6a23c] leading-relaxed">
            注意：此套餐前台不可见、不可购买，仅通过后台邀请码兑换或管理员发放。
            <strong className="text-[#f56c6c]">它是真无限权益套餐，资源额度显示为不限量，但必须严格受安全风控、违法内容审核、DDoS/WAF 保护规则约束。</strong>
            滥用此套餐可能导致节点资源枯竭。
          </p>
        </div>
      </div>

      <Card title="Infinite Max 权益配置" extra={<Button type="primary" onClick={() => setShowConfirm(true)}>主动发放给用户</Button>}>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-x-10 gap-y-2">
          <FormItem label="内部标识"><Input value="infinite-max" disabled /></FormItem>
          <FormItem label="套餐别名"><Input value="Infinite Max (企业赞助/内部测试版)" disabled /></FormItem>
          <FormItem label="存储配额"><Input value="不限量 (Unlimited)" disabled /></FormItem>
          <FormItem label="流量配额"><Input value="不限量 (Unlimited)" disabled /></FormItem>
          <FormItem label="API 频率限制" helpText="即使不限量，也必须保留底层 API 限流防 CC 攻击"><Input value="100,000 次 / 分钟" disabled /></FormItem>
          <FormItem label="单文件大小限制" helpText="覆盖全局默认设置"><Input value="100 MB" disabled /></FormItem>
        </div>
        <div className="mt-6 border-t pt-4">
          <h5 className="font-semibold mb-3 flex items-center gap-2"><ShieldAlert className="w-4 h-4 text-[#f56c6c]" /> 强制安全覆盖策略 (不可取消)</h5>
          <div className="flex gap-4 flex-wrap">
            <Tag type="danger">违法图片自动冻结</Tag>
            <Tag type="danger">WAF 恶意流量清洗</Tag>
            <Tag type="danger">异常账单/流量突增告警</Tag>
            <Tag type="danger">多地异常登录拦截</Tag>
          </div>
        </div>
      </Card>

      <Dialog
        visible={showConfirm}
        title="高风险操作确认：发放 Infinite Max"
        danger
        onClose={() => { if (!submitting) setShowConfirm(false); }}
        onConfirm={() => void grantInfiniteMax()}
        confirmText={submitting ? "发放中..." : "确认发放并记录审计"}
        confirmDisabled={submitting}
      >
        <div className="space-y-4">
          <p>您即将向指定用户发放 <strong>Infinite Max</strong> 无限额度套餐。</p>
          <FormItem label="目标用户 ID/邮箱" required><Input value={target} onChange={setTarget} placeholder="输入精确的 ID 或邮箱" /></FormItem>
          <FormItem label="发放时长" required>
            <select value={grantDays} onChange={(event) => setGrantDays(event.target.value)} className="w-full border border-[#dcdfe6] rounded px-3 py-2 bg-white">
              <option value="30">1 个月 (测试)</option>
              <option value="365">1 年</option>
              <option value="3650">10 年 (极高风险)</option>
            </select>
          </FormItem>
          <FormItem label="发放事由" required><Input type="text" value={reason} onChange={setReason} placeholder="必须填写，将写入审计日志" /></FormItem>
          <div className="p-3 bg-[#fef0f0] border border-[#fde2e2] rounded text-[#f56c6c] text-xs">
            系统将自动监控该账户的流量和存储突变情况。如果检测到滥用行为，系统管理员有权随时强制冻结该账户。
          </div>
        </div>
      </Dialog>
    </div>
  );
};

const InviteCampaignView = () => {
  const [name, setName] = useState("2026 开发者增长活动");
  const [planSlug, setPlanSlug] = useState("pro");
  const [code, setCode] = useState("");
  const [grantDays, setGrantDays] = useState("30");
  const [totalLimit, setTotalLimit] = useState("100");
  const [perUserLimit, setPerUserLimit] = useState("1");
  const [perIPLimit, setPerIPLimit] = useState("1");
  const [strictDevice, setStrictDevice] = useState(true);
  const [newUsersOnly, setNewUsersOnly] = useState(true);
  const [requireEmailVerified, setRequireEmailVerified] = useState(true);
  const [requireOAuthBinding, setRequireOAuthBinding] = useState(false);
  const [requireAdminApproval, setRequireAdminApproval] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [plans, setPlans] = useState<AdminPlan[]>([]);

  useEffect(() => {
    adminFetch<AdminPlan[]>("/admin/plans")
      .then((rows) => {
        setPlans(rows);
        if (!rows.some((plan) => plan.slug === planSlug)) {
          const fallbackPlan = rows.find((plan) => plan.slug === "pro") || rows.find((plan) => plan.visibility === "visible") || rows[0];
          if (fallbackPlan) setPlanSlug(fallbackPlan.slug);
        }
      })
      .catch((error) => {
        setNotice({ type: "danger", text: error instanceof Error ? error.message : "套餐列表读取失败，邀请活动暂时无法选择套餐。" });
      });
  }, []);

  const reset = () => {
    setName("2026 开发者增长活动");
    setPlanSlug("pro");
    setCode("");
    setGrantDays("30");
    setTotalLimit("100");
    setPerUserLimit("1");
    setPerIPLimit("1");
    setStrictDevice(true);
    setNewUsersOnly(true);
    setRequireEmailVerified(true);
    setRequireOAuthBinding(false);
    setRequireAdminApproval(false);
    setNotice(null);
  };

  const createInvite = async () => {
    if (submitting) return;
    if (!name.trim()) {
      setNotice({ type: "danger", text: "请填写活动名称。" });
      return;
    }
    if (!planSlug) {
      setNotice({ type: "danger", text: "请选择赠送套餐。" });
      return;
    }
    setSubmitting(true);
    try {
      const invite = await adminFetch<{ code: string; plan_slug: string; grant_days: number }>("/admin/invites", {
        method: "POST",
        body: JSON.stringify({
          name: name.trim(),
          code: code.trim(),
          plan_slug: planSlug,
          grant_days: Number(grantDays) || 30,
          total_limit: Number(totalLimit) || 100,
          per_user_limit: Number(perUserLimit) || 1,
          per_email_limit: 1,
          per_ip_limit: Number(perIPLimit) || 1,
          per_device_limit: strictDevice ? 1 : 0,
          new_users_only: newUsersOnly,
          require_email_verified: requireEmailVerified,
          require_oauth_binding: requireOAuthBinding,
          require_admin_approval: requireAdminApproval,
          allow_stacking: false,
          status: "active",
          notes: planSlug === "infinite-max" ? "高风险隐藏套餐邀请，必须配合人工审计和风控监控。" : "运营后台创建的邀请活动。",
        }),
      });
      setNotice({ type: "success", text: `邀请活动已生成：${invite.code}，套餐 ${invite.plan_slug}，有效权益 ${invite.grant_days} 天。` });
    } catch (error) {
      setNotice({ type: "danger", text: error instanceof Error ? error.message : "创建失败，请检查后台服务。" });
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="max-w-4xl">
      <Card title="创建邀请与兑换活动">
        <div className="space-y-6">
          <NoticeBar notice={notice} />
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-x-8 gap-y-4">
            <FormItem label="活动名称" required><Input value={name} onChange={setName} placeholder="例如：2026春节开发者特惠" /></FormItem>
            <FormItem label="赠送套餐" required>
              <select value={planSlug} onChange={(event) => {
                setPlanSlug(event.target.value);
                if (event.target.value === "infinite-max") setRequireAdminApproval(true);
              }} className="w-full border border-[#dcdfe6] rounded px-3 py-2 text-sm bg-white">
                {plans.map((plan) => (
                  <option key={plan.slug} value={plan.slug}>
                    {plan.name}{plan.visibility === "hidden" ? " (隐藏)" : ""}{plan.slug === "infinite-max" ? " (需审批)" : ""}
                  </option>
                ))}
                {!plans.length && <option value={planSlug}>{planSlug || "加载套餐中..."}</option>}
              </select>
            </FormItem>
            <FormItem label="兑换码/链接后缀" helpText="留空则系统自动生成唯一 Hash"><Input value={code} onChange={setCode} placeholder="自定义短码，如: SPRING2026" /></FormItem>
            <FormItem label="赠送时长" required><Input type="number" value={grantDays} onChange={setGrantDays} placeholder="天数，例如：30" /></FormItem>
          </div>

          <div className="border-t border-[#ebeef5] pt-5 mt-5">
            <h4 className="text-sm font-semibold mb-4 text-[#303133]">风控与领取限制策略</h4>
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-x-8 gap-y-4">
              <FormItem label="总领取次数限制"><Input type="number" value={totalLimit} onChange={setTotalLimit} placeholder="默认 100" /></FormItem>
              <FormItem label="单账号限领次数"><Input type="number" value={perUserLimit} onChange={setPerUserLimit} placeholder="默认 1" /></FormItem>
              <FormItem label="单 IP 限领次数" helpText="防刷黑产保护"><Input type="number" value={perIPLimit} onChange={setPerIPLimit} placeholder="默认 1" /></FormItem>
              <FormItem label="设备指纹限制" helpText="基于 Canvas/WebGL 指纹">
                <select value={strictDevice ? "strict" : "loose"} onChange={(event) => setStrictDevice(event.target.value === "strict")} className="w-full border rounded px-3 py-2 text-sm bg-white">
                  <option value="strict">严格模式 (推荐)</option>
                  <option value="loose">宽松模式</option>
                </select>
              </FormItem>
            </div>
          </div>

          <div className="border-t border-[#ebeef5] pt-5 mt-5">
            <h4 className="text-sm font-semibold mb-4 text-[#303133]">前置校验条件</h4>
            <div className="space-y-3 pl-4">
              <label className="flex items-center gap-2 text-sm"><input type="checkbox" className="rounded" checked={newUsersOnly} onChange={(event) => setNewUsersOnly(event.target.checked)} /> 仅限新注册用户领取</label>
              <label className="flex items-center gap-2 text-sm"><input type="checkbox" className="rounded" checked={requireEmailVerified} onChange={(event) => setRequireEmailVerified(event.target.checked)} /> 必须完成邮箱验证</label>
              <label className="flex items-center gap-2 text-sm"><input type="checkbox" className="rounded" checked={requireOAuthBinding} onChange={(event) => setRequireOAuthBinding(event.target.checked)} /> 必须绑定 Github/Google OAuth</label>
              <label className="flex items-center gap-2 text-sm"><input type="checkbox" className="rounded" checked={requireAdminApproval} onChange={(event) => setRequireAdminApproval(event.target.checked)} /> 领取后需要管理员手动审批生效</label>
            </div>
          </div>

          <div className="flex justify-end gap-3 pt-6">
            <Button onClick={reset} disabled={submitting}>取消重置</Button>
            <Button type="primary" icon={Plus} disabled={submitting} onClick={() => void createInvite()}>{submitting ? "生成中..." : "生成邀请活动"}</Button>
          </div>
        </div>
      </Card>
    </div>
  );
};

const ImageModView = () => {
  const [images, setImages] = useState<Row[]>([]);
  const [keyword, setKeyword] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [total, setTotal] = useState(0);
  const [notice, setNotice] = useState<Notice | null>(null);
  const [processingID, setProcessingID] = useState("");
  const loadImages = () => {
    const params = new URLSearchParams({ limit: "200" });
    if (keyword.trim()) params.set("q", keyword.trim());
    if (statusFilter) params.set("status", statusFilter);
    adminFetch<{ items: Array<{ public_id: string; user_id: string; filename: string; bytes: number; status: string; perceptual_hash: string; moderation_reason?: string }>; total: number }>(`/admin/images?${params.toString()}`)
      .then((payload) => {
        setNotice(null);
        setTotal(payload.total);
        setImages(payload.items.map((image) => ({
          id: image.public_id,
          user: image.user_id,
          filename: image.filename,
          size: formatBytes(image.bytes),
          status: image.status,
          hash: image.perceptual_hash || "-",
          reason: image.moderation_reason || "-",
        })));
      })
      .catch((error) => {
        setImages([]);
        setTotal(0);
        setNotice({ type: "danger", text: error instanceof Error ? error.message : "图片审核队列读取失败，请检查 API 服务。" });
      });
  };
  useEffect(loadImages, []);
  const freezeImage = async (id: string) => {
    if (processingID) return;
    setProcessingID(id);
    try {
      await adminFetch(`/admin/images/${id}/freeze`, { method: "POST", body: JSON.stringify({ reason: "人工审核冻结" }) });
      setNotice({ type: "success", text: `对象 ${id} 已冻结。` });
      loadImages();
    } catch (error) {
      setNotice({ type: "danger", text: error instanceof Error ? error.message : "冻结失败，请检查后台服务。" });
    } finally {
      setProcessingID("");
    }
  };
  const deleteImage = async (id: string) => {
    if (processingID) return;
    setProcessingID(id);
    try {
      await adminFetch(`/admin/images/${id}`, { method: "DELETE" });
      setNotice({ type: "success", text: `对象 ${id} 已删除。` });
      loadImages();
    } catch (error) {
      setNotice({ type: "danger", text: error instanceof Error ? error.message : "删除失败，请检查后台服务。" });
    } finally {
      setProcessingID("");
    }
  };
  return (
    <Card title="图片安全审核队列" extra={<Tag type={total > 0 ? "warning" : "success"}>命中 {total} 个对象</Tag>}>
      <NoticeBar notice={notice} className="mb-4" />
      <div className="flex flex-wrap gap-4 mb-4">
        <Input value={keyword} onChange={setKeyword} placeholder="Public ID / 用户 / pHash / 原因" prefix={Search} className="w-72" />
        <select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)} className="border border-[#dcdfe6] rounded px-3 py-2 text-sm bg-white">
          <option value="">处理状态: 全部</option>
          <option value="active">active</option>
          <option value="frozen">frozen</option>
          <option value="deleted">deleted</option>
        </select>
        <div className="flex-1" />
        <Button icon={Search} type="primary" onClick={loadImages}>搜索</Button>
        <Button icon={RefreshCw} onClick={loadImages}>刷新队列</Button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 lg:grid-cols-4 gap-4">
        {images.map((image) => (
          <div key={image.id} className="border border-[#e4e7ed] rounded-md overflow-hidden bg-[#fafafa]">
            <div className="h-40 bg-[#ebeef5] flex items-center justify-center relative group">
              <ImageIcon className="w-10 h-10 text-[#c0c4cc]" />
              <div className="absolute inset-0 bg-black/60 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center gap-2 backdrop-blur-sm">
                <Button size="small" type="warning" disabled={Boolean(processingID)} onClick={() => void freezeImage(String(image.id))}>{processingID === image.id ? "处理中" : "冻结"}</Button>
                <Button size="small" type="danger" disabled={Boolean(processingID)} onClick={() => void deleteImage(String(image.id))}>{processingID === image.id ? "处理中" : "删除"}</Button>
              </div>
              <div className="absolute top-2 right-2"><Tag type={image.status === "active" ? "success" : image.status === "frozen" ? "warning" : "danger"}>{image.status}</Tag></div>
            </div>
            <div className="p-3 text-xs space-y-1 text-[#606266]">
              <div className="flex justify-between"><span>图片 ID:</span> <span className="font-mono">{image.id}</span></div>
              <div className="flex justify-between"><span>上传者:</span> <span className="text-[#409eff] cursor-pointer truncate max-w-32">{image.user}</span></div>
              <div className="flex justify-between"><span>大小:</span> <span className="font-mono">{image.size}</span></div>
              <div className="flex justify-between"><span>感知哈希:</span> <span className="font-mono truncate w-24">{image.hash}</span></div>
              <div className="flex justify-between text-[#e6a23c]"><span>处置原因:</span> <span className="truncate w-28 text-right">{image.reason}</span></div>
            </div>
          </div>
        ))}
        {images.length === 0 && <div className="text-sm text-[#909399] p-8">暂无待审核图片。</div>}
      </div>
    </Card>
  );
};

const SecurityView = () => {
  const [events, setEvents] = useState<Row[]>([]);
  const [systemConfig, setSystemConfig] = useState<AdminSystemConfig | null>(null);
  const [notice, setNotice] = useState("");
  useEffect(() => {
    adminFetch<Array<{ created_at: string; type: string; message: string; ip: string; referer: string }>>("/admin/security/events")
      .then((rows) => {
        setNotice("");
        setEvents(rows.map((event) => ({
          time: event.created_at?.replace("T", " ").slice(11, 19) || "-",
          type: event.type,
          ip: event.ip || "-",
          rule: event.message,
          action: event.referer ? `Referer: ${event.referer}` : "已记录风险事件",
        })));
      })
      .catch((error) => {
        setEvents([]);
        setNotice(error instanceof Error ? error.message : "风控事件读取失败，请检查 API 服务。");
      });
    adminFetch<AdminSystemConfig>("/admin/system/config")
      .then(setSystemConfig)
      .catch((error) => {
        setSystemConfig(null);
        setNotice(error instanceof Error ? error.message : "系统限流配置读取失败，请检查 API 服务。");
      });
  }, []);
  const uniqueIPs = new Set(events.map((event) => event.ip).filter((ip) => ip && ip !== "-")).size;
  const hotlinkEvents = events.filter((event) => String(event.type).includes("hotlink")).length;
  const rateLimitEvents = events.filter((event) => String(event.type).includes("rate")).length;
  return (
    <div className="space-y-6">
    <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
      <MetricCard title="风险事件总数" value={String(events.length)} icon={ShieldAlert} colorClass="bg-red-500" />
      <MetricCard title="防盗链触发" value={String(hotlinkEvents)} icon={Activity} colorClass="bg-orange-500" />
      <MetricCard title="限流触发" value={String(rateLimitEvents)} icon={Lock} colorClass="bg-yellow-500" />
    </div>

    <Card title="风控事件看板 (实时)">
      {notice && <div className="mb-4 bg-[#fef0f0] border border-[#fde2e2] text-[#f56c6c] px-4 py-3 rounded text-sm">{notice}</div>}
      <div className="mb-3 text-xs text-[#909399]">已识别来源 IP：{uniqueIPs} 个。无事件时表示当前没有被后端记录的风险命中。</div>
      <Table
        columns={[
          { title: "时间", dataIndex: "time" },
          { title: "事件类型", dataIndex: "type", render: (val) => <Tag type={String(val).includes("攻击") ? "danger" : "warning"}>{val}</Tag> },
          { title: "源 IP", dataIndex: "ip", render: (val) => <span className="font-mono text-xs">{val}</span> },
          { title: "命中规则", dataIndex: "rule", render: (val) => <span className="text-[#f56c6c] font-medium">{val}</span> },
          { title: "执行动作", dataIndex: "action" },
        ]}
        data={events}
      />
    </Card>

    <Card title="全局风控阈值配置" extra={<Tag type="info">只读运行参数</Tag>}>
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        {[
          ["默认接口", systemConfig?.rate_limits?.default_per_minute],
          ["图片上传", systemConfig?.rate_limits?.image_upload_per_minute],
          ["支付下单", systemConfig?.rate_limits?.ifpay_checkout_per_minute],
          ["登录接口", systemConfig?.rate_limits?.login_per_minute],
        ].map(([label, value]) => (
          <div key={label} className="border border-[#ebeef5] rounded p-4 bg-[#f5f7fa]">
            <div className="text-xs text-[#909399] mb-1">{label}</div>
            <div className="text-xl font-bold font-mono text-[#303133]">{String(value ?? "-")}</div>
            <div className="text-xs text-[#909399] mt-1">次 / 分钟 / IP</div>
          </div>
        ))}
      </div>
      <div className="mt-4 pt-4 border-t text-xs text-[#909399]">当前阈值由后端运行时中间件执行，超过阈值会写入风险事件并返回 429。</div>
    </Card>
    </div>
  );
};

const BackupView = () => {
  const [notice, setNotice] = useState<{ type: ThemeTag; text: string } | null>(null);
  const [validation, setValidation] = useState<Row | null>(null);
  const [fileName, setFileName] = useState("");
  const [exporting, setExporting] = useState(false);
  const [validating, setValidating] = useState(false);

  const exportBackup = async () => {
    if (exporting) return;
    setExporting(true);
    try {
      const res = await adminBinaryFetch("/admin/backups/export");
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `yuexiang-admin-backup-${new Date().toISOString().slice(0, 10)}.zip`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
      setNotice({ type: "success", text: "全量备份包已生成，包含数据库快照、对象文件、manifest 与 checksum。" });
    } catch (error) {
      setNotice({ type: "danger", text: error instanceof Error ? error.message : "导出失败，请检查后台服务。" });
    } finally {
      setExporting(false);
    }
  };

  const validateBackup = async (file: File | null) => {
    if (!file || validating) return;
    setFileName(file.name);
    setValidating(true);
    const form = new FormData();
    form.append("file", file);
    try {
      const result = await adminFetch<Row>("/admin/backups/import/validate", { method: "POST", body: form });
      setValidation(result);
      setNotice({ type: "success", text: `备份包预检通过：${result.file_count} 个文件，${result.object_count} 个对象。` });
    } catch (error) {
      setValidation(null);
      setNotice({ type: "danger", text: error instanceof Error ? error.message : "预检失败，请检查备份包。" });
    } finally {
      setValidating(false);
    }
  };

  return (
    <div className="space-y-6">
      {notice && (
        <div className={`border px-4 py-3 rounded text-sm ${notice.type === "success" ? "bg-[#f0f9eb] border-[#e1f3d8] text-[#67c23a]" : "bg-[#fef0f0] border-[#fde2e2] text-[#f56c6c]"}`}>
          {notice.text}
        </div>
      )}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <Card title="全量数据导出" extra={<Tag type="success">服务空闲</Tag>}>
          <p className="text-sm text-[#606266] mb-6 leading-relaxed">
            将导出所有数据库记录、图片对象、派生图、审计日志、风险事件、配置快照与完整性校验文件。密钥不会写入备份包。
          </p>
          <div className="space-y-3 mb-6">
            <label className="flex items-center gap-2 text-sm"><input type="checkbox" className="rounded" checked readOnly /> 包含已软删除记录和审计日志</label>
            <label className="flex items-center gap-2 text-sm"><input type="checkbox" className="rounded" checked readOnly /> 包含 manifest.json 与 checksums.sha256</label>
          </div>
          <Button type="primary" icon={Download} className="w-full" disabled={exporting} onClick={() => void exportBackup()}>{exporting ? "生成中..." : "生成全量 ZIP 备份包"}</Button>
          <div className="mt-4 text-xs text-[#909399]">导出包可用于人工迁移、合规审计与异地灾备演练。</div>
        </Card>

        <Card title="系统灾备恢复" extra={<Tag type="danger">高危操作</Tag>}>
          <label className="block border-2 border-dashed border-[#dcdfe6] rounded-md p-8 text-center hover:border-[#409eff] hover:bg-[#ecf5ff] transition-colors cursor-pointer mb-6">
            <UploadCloud className="w-10 h-10 text-[#c0c4cc] mx-auto mb-2" />
            <div className="text-sm font-medium text-[#303133]">{validating ? "预检中..." : fileName || "选择备份 ZIP 文件进行预检"}</div>
            <div className="text-xs text-[#909399] mt-1">校验 manifest.json、checksums.sha256 与对象数量，不会覆盖线上数据</div>
            <input type="file" accept=".zip,application/zip" disabled={validating} className="hidden" onChange={(event) => void validateBackup(event.target.files?.[0] || null)} />
          </label>

          <div className="bg-[#fdf6ec] border border-[#faecd8] p-3 rounded text-sm text-[#e6a23c] mb-4 flex items-start gap-2">
            <AlertTriangle className="w-4 h-4 flex-shrink-0 mt-0.5" />
            <div>生产恢复必须在维护窗口执行。当前控制台只开放安全预检，真正覆盖恢复应由 DBA/运维在离线环境确认后执行。</div>
          </div>
          <div className="grid grid-cols-3 gap-3 text-center text-xs">
            <div className="bg-[#f5f7fa] border border-[#ebeef5] rounded p-3"><div className="font-mono text-[#303133]">{String(validation?.file_count ?? "-")}</div><div className="text-[#909399]">文件数</div></div>
            <div className="bg-[#f5f7fa] border border-[#ebeef5] rounded p-3"><div className="font-mono text-[#303133]">{String(validation?.object_count ?? "-")}</div><div className="text-[#909399]">对象数</div></div>
            <div className="bg-[#f5f7fa] border border-[#ebeef5] rounded p-3"><div className="font-mono text-[#303133]">{validation?.checksum_found ? "YES" : "-"}</div><div className="text-[#909399]">Checksum</div></div>
          </div>
        </Card>
      </div>
    </div>
  );
};

const OrdersView = () => {
  const [orders, setOrders] = useState<Row[]>([]);
  const [notice, setNotice] = useState<{ type: ThemeTag; text: string } | null>(null);
  const [actionTarget, setActionTarget] = useState<Row | null>(null);
  const [actionKind, setActionKind] = useState<"mark-paid" | "cancel" | "refund">("mark-paid");
  const [actionReason, setActionReason] = useState("");
  const [submittingAction, setSubmittingAction] = useState(false);
  const [keyword, setKeyword] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const loadOrders = () => {
    adminFetch<AdminOrder[]>("/admin/orders")
      .then((rows) => setOrders(rows.map((order) => ({
        id: order.id,
        user: order.user_id,
        plan: order.plan_slug,
        cycle: order.billing_cycle,
        amount: `¥ ${(order.amount_cent / 100).toFixed(2)}`,
        status: order.status,
        payment: order.ifpay_payment_id || "-",
        createdAt: order.created_at?.replace("T", " ").slice(0, 19) || "-",
        paidAt: order.paid_at?.replace("T", " ").slice(0, 19) || "-",
        failedAt: order.failed_at?.replace("T", " ").slice(0, 19) || "-",
        note: order.operator_note || "-",
      }))))
      .catch((error) => {
        setOrders([]);
        setNotice({ type: "danger", text: error instanceof Error ? error.message : "订单列表读取失败，请检查 API 服务。" });
      });
  };
  useEffect(loadOrders, []);
  const openOrderAction = (row: Row, kind: "mark-paid" | "cancel" | "refund") => {
    setActionTarget(row);
    setActionKind(kind);
    const label = kind === "mark-paid" ? "人工入账对账" : kind === "cancel" ? "取消未支付订单" : "退款并终止权益";
    setActionReason(`${label}：${row.id}`);
  };
  const submitOrderAction = async () => {
    if (!actionTarget || submittingAction) return;
    setSubmittingAction(true);
    try {
      await adminFetch(`/admin/orders/${encodeURIComponent(String(actionTarget.id))}/${actionKind}`, {
        method: "POST",
        body: JSON.stringify({ reason: actionReason.trim() }),
      });
      setNotice({ type: "success", text: "订单操作已完成，并写入审计日志。" });
      setActionTarget(null);
      setActionReason("");
      loadOrders();
    } catch (error) {
      setNotice({ type: "danger", text: error instanceof Error ? error.message : "订单操作失败，请检查后台服务。" });
    } finally {
      setSubmittingAction(false);
    }
  };
  const filteredOrders = orders.filter((order) => {
    const query = keyword.trim().toLowerCase();
    const matchesStatus = !statusFilter || order.status === statusFilter;
    if (!matchesStatus) return false;
    if (!query) return true;
    return [order.id, order.user, order.plan, order.status, order.payment, order.note].some((value) => String(value).toLowerCase().includes(query));
  });
  const exportOrders = () => {
    const rows = [["id", "user_id", "plan", "cycle", "amount", "status", "payment", "paid_at", "failed_at", "created_at", "note"], ...filteredOrders.map((order) => [
      order.id,
      order.user,
      order.plan,
      order.cycle,
      order.amount,
      order.status,
      order.payment,
      order.paidAt,
      order.failedAt,
      order.createdAt,
      order.note,
    ])];
    downloadText("yuexiang-orders.csv", rows.map((row) => row.map((cell) => `"${String(cell).replaceAll("\"", "\"\"")}"`).join(",")).join("\n"));
  };
  return (
    <Card title="订单与订阅" extra={<Tag type="primary">IF-Pay</Tag>}>
      {notice && <div className={`mb-4 border px-4 py-3 rounded text-sm ${notice.type === "success" ? "bg-[#f0f9eb] border-[#e1f3d8] text-[#67c23a]" : "bg-[#fef0f0] border-[#fde2e2] text-[#f56c6c]"}`}>{notice.text}</div>}
      <div className="flex flex-wrap gap-4 mb-4">
        <Input value={keyword} onChange={setKeyword} placeholder="订单/用户/支付单/备注" prefix={Search} className="w-72" />
        <select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)} className="border border-[#dcdfe6] rounded px-3 py-2 text-sm bg-white">
          <option value="">全部状态</option>
          <option value="pending">pending</option>
          <option value="paid">paid</option>
          <option value="failed">failed</option>
          <option value="cancelled">cancelled</option>
          <option value="refunded">refunded</option>
        </select>
        <div className="flex-1" />
        <Button icon={Download} onClick={exportOrders}>导出对账 CSV</Button>
      </div>
      <Table
        columns={[
          { title: "订单 ID", dataIndex: "id", render: (val) => <span className="font-mono text-xs">{val}</span> },
          { title: "用户", dataIndex: "user", render: (val) => <span className="font-mono text-xs text-[#409eff]">{val}</span> },
          { title: "套餐", dataIndex: "plan", render: (val) => <Tag type={val === "infinite-max" ? "warning" : "primary"}>{val}</Tag> },
          { title: "周期", dataIndex: "cycle" },
          { title: "金额", dataIndex: "amount" },
          { title: "状态", dataIndex: "status", render: (val) => <Tag type={val === "paid" ? "success" : val === "pending" ? "warning" : val === "failed" || val === "refunded" ? "danger" : "info"}>{val}</Tag> },
          { title: "IF-Pay Payment", dataIndex: "payment", render: (val) => <span className="font-mono text-xs">{val}</span> },
          { title: "支付时间", dataIndex: "paidAt", render: (val) => <span className="text-xs">{val}</span> },
          { title: "失败时间", dataIndex: "failedAt", render: (val) => <span className="text-xs">{val}</span> },
          { title: "创建时间", dataIndex: "createdAt", render: (val) => <span className="text-xs">{val}</span> },
          { title: "操作", dataIndex: "id", render: (_, row) => (
            <div className="flex gap-2">
              {row.status === "pending" && <Button type="text" size="small" onClick={() => openOrderAction(row, "mark-paid")}>入账</Button>}
              {row.status === "pending" && <Button type="text" size="small" className="text-[#909399]" onClick={() => openOrderAction(row, "cancel")}>取消</Button>}
              {row.status === "paid" && <Button type="text" size="small" className="text-[#f56c6c] hover:text-[#f78989]" onClick={() => openOrderAction(row, "refund")}>退款</Button>}
            </div>
          ) },
        ]}
        data={filteredOrders}
      />
      <Dialog
        visible={Boolean(actionTarget)}
        title={actionKind === "mark-paid" ? "人工入账确认" : actionKind === "cancel" ? "取消订单确认" : "退款确认"}
        danger={actionKind !== "mark-paid"}
        confirmText={submittingAction ? "处理中..." : actionKind === "mark-paid" ? "确认入账" : actionKind === "cancel" ? "确认取消" : "确认退款"}
        confirmDisabled={submittingAction || !actionReason.trim()}
        onClose={() => { if (!submittingAction) { setActionTarget(null); setActionReason(""); } }}
        onConfirm={() => void submitOrderAction()}
      >
        <div className="space-y-4">
          <p>订单 <strong className="font-mono">{String(actionTarget?.id || "")}</strong> 将执行 <strong>{actionKind}</strong> 操作，原因会写入审计日志。</p>
          <FormItem label="操作原因" required>
            <Input value={actionReason} onChange={setActionReason} placeholder="请输入对账/取消/退款原因" />
          </FormItem>
        </div>
      </Dialog>
    </Card>
  );
};

const InviteRecordsView = () => {
  const [records, setRecords] = useState<Row[]>([]);
  const [notice, setNotice] = useState("");
  useEffect(() => {
    adminFetch<{ redemptions: Array<{ id: string; code: string; user_id: string; email: string; ip: string; device_id: string; plan_slug: string; created_at: string }> }>("/admin/invites")
      .then((data) => {
        setNotice("");
        setRecords(data.redemptions.map((item) => ({
          id: item.id,
          code: item.code,
          user: item.user_id,
          email: item.email,
          ip: item.ip,
          device: item.device_id || "-",
          plan: item.plan_slug,
          createdAt: item.created_at?.replace("T", " ").slice(0, 19) || "-",
        })));
      })
      .catch((error) => {
        setRecords([]);
        setNotice(error instanceof Error ? error.message : "邀请兑换记录读取失败，请检查 API 服务。");
      });
  }, []);
  return (
    <Card title="邀请兑换记录" extra={<Tag type="info">实时风控记录</Tag>}>
      {notice && <div className="mb-4 bg-[#fef0f0] border border-[#fde2e2] text-[#f56c6c] px-4 py-3 rounded text-sm">{notice}</div>}
      <Table
        columns={[
          { title: "记录 ID", dataIndex: "id", render: (val) => <span className="font-mono text-xs">{val}</span> },
          { title: "邀请码", dataIndex: "code", render: (val) => <Tag type={val === "infinite-max-internal" ? "warning" : "primary"}>{val}</Tag> },
          { title: "用户", dataIndex: "user", render: (val) => <span className="font-mono text-xs">{val}</span> },
          { title: "邮箱", dataIndex: "email" },
          { title: "IP", dataIndex: "ip", render: (val) => <span className="font-mono text-xs">{val}</span> },
          { title: "设备指纹", dataIndex: "device", render: (val) => <span className="font-mono text-xs">{val}</span> },
          { title: "套餐", dataIndex: "plan" },
          { title: "兑换时间", dataIndex: "createdAt", render: (val) => <span className="text-xs">{val}</span> },
        ]}
        data={records}
      />
    </Card>
  );
};

const StorageConfigView = () => {
  const [config, setConfig] = useState<Row | null>(null);
  const [notice, setNotice] = useState("");
  useEffect(() => {
    adminFetch<Row>("/admin/storage/config")
      .then((data) => {
        setNotice("");
        setConfig(data);
      })
      .catch((error) => {
        setConfig(null);
        setNotice(error instanceof Error ? error.message : "对象存储配置读取失败，请检查 API 服务。");
      });
  }, []);
  return (
    <Card title="对象存储配置" extra={<Tag type={config?.bucket ? "success" : "warning"}>{config?.bucket ? "已连接" : "待配置"}</Tag>}>
      {notice && <div className="mb-4 bg-[#fef0f0] border border-[#fde2e2] text-[#f56c6c] px-4 py-3 rounded text-sm">{notice}</div>}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
        {[
          ["Endpoint", config?.endpoint || "-"],
          ["Region", config?.region || "-"],
          ["Bucket", config?.bucket || "-"],
          ["Path Style", String(config?.force_path_style ?? "-")],
        ].map(([label, value]) => (
          <div key={label} className="border border-[#ebeef5] rounded p-4 bg-[#f5f7fa]/60">
            <div className="text-xs text-[#909399] mb-1">{label}</div>
            <div className="font-mono text-[#303133] break-all">{value}</div>
          </div>
        ))}
      </div>
      <div className="mt-4 p-3 bg-[#ecf5ff] border border-[#d9ecff] rounded text-xs text-[#409eff]">
        生产建议：Bucket 保持私有，图片统一通过 `/i/*` 鉴权入口分发；Cloudflare 只负责缓存与 WAF，不能替代后端签名/Referer 策略。
      </div>
    </Card>
  );
};

const HotlinkConfigView = () => {
  const [config, setConfig] = useState<{ allowed_domains?: string[]; blocked_domains?: string[]; allow_empty_referer?: boolean; signing_enabled?: boolean; updated_at?: string } | null>(null);
  const [allowed, setAllowed] = useState("");
  const [blocked, setBlocked] = useState("");
  const [allowEmpty, setAllowEmpty] = useState(true);
  const [notice, setNotice] = useState<Notice | null>(null);
  const [saving, setSaving] = useState(false);
  const loadHotlink = () => {
    adminFetch<{ allowed_domains: string[]; blocked_domains: string[]; allow_empty_referer: boolean; signing_enabled: boolean }>("/admin/security/hotlink")
      .then((data) => {
        setConfig(data);
        setAllowed((data.allowed_domains || []).join("\n"));
        setBlocked((data.blocked_domains || []).join("\n"));
        setAllowEmpty(data.allow_empty_referer);
      })
      .catch((error) => {
        setConfig(null);
        setNotice({ type: "danger", text: error instanceof Error ? error.message : "防盗链策略读取失败，请检查 API 服务。" });
      });
  };
  useEffect(() => {
    loadHotlink();
  }, []);
  const parseDomains = (value: string) => value.split(/[\n,]/).map((item) => item.trim()).filter(Boolean);
  const saveHotlink = async () => {
    if (saving) return;
    setSaving(true);
    try {
      const data = await adminFetch<{ allowed_domains: string[]; blocked_domains: string[]; allow_empty_referer: boolean; signing_enabled: boolean; updated_at?: string }>("/admin/security/hotlink", {
        method: "PATCH",
        body: JSON.stringify({
          allowed_domains: parseDomains(allowed),
          blocked_domains: parseDomains(blocked),
          allow_empty_referer: allowEmpty,
        }),
      });
      setConfig(data);
      setAllowed((data.allowed_domains || []).join("\n"));
      setBlocked((data.blocked_domains || []).join("\n"));
      setAllowEmpty(data.allow_empty_referer);
      setNotice({ type: "success", text: "防盗链策略已实时保存，图片分发入口会立即使用新规则。" });
    } catch (error) {
      setNotice({ type: "danger", text: error instanceof Error ? error.message : "保存失败，请检查后台服务。" });
    } finally {
      setSaving(false);
    }
  };
  return (
    <Card title="防盗链策略" extra={<Tag type={config?.signing_enabled ? "success" : "warning"}>{config?.signing_enabled ? "签名启用" : "签名未启用"}</Tag>}>
      <NoticeBar notice={notice} className="mb-4" />
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <div className="border border-[#e4e7ed] rounded p-4 bg-[#f5f7fa]">
          <div className="text-sm font-semibold text-[#303133] mb-3">允许 Referer 域名</div>
          <textarea value={allowed} onChange={(event) => setAllowed(event.target.value)} className="w-full h-28 border border-[#dcdfe6] rounded px-3 py-2 text-xs font-mono focus:outline-none focus:border-[#409eff]" placeholder={"example.com\n*.example.com"} />
          <div className="flex flex-wrap gap-2">
            {(config?.allowed_domains || []).map((domain) => <Tag key={domain} type="success">{domain}</Tag>)}
            {!config?.allowed_domains?.length && <span className="text-xs text-[#909399]">未配置</span>}
          </div>
        </div>
        <div className="border border-[#e4e7ed] rounded p-4 bg-[#f5f7fa]">
          <div className="text-sm font-semibold text-[#303133] mb-3">阻断 Referer 域名</div>
          <textarea value={blocked} onChange={(event) => setBlocked(event.target.value)} className="w-full h-28 border border-[#dcdfe6] rounded px-3 py-2 text-xs font-mono focus:outline-none focus:border-[#409eff]" placeholder={"bad-site.com\n盗链来源域名"} />
          <div className="flex flex-wrap gap-2">
            {(config?.blocked_domains || []).map((domain) => <Tag key={domain} type="danger">{domain}</Tag>)}
            {!config?.blocked_domains?.length && <span className="text-xs text-[#909399]">未配置</span>}
          </div>
        </div>
        <div className="border border-[#e4e7ed] rounded p-4 bg-[#f5f7fa]">
          <div className="text-sm font-semibold text-[#303133] mb-3">空 Referer 策略</div>
          <label className="flex items-center gap-2 text-sm mb-3">
            <input type="checkbox" checked={allowEmpty} onChange={(event) => setAllowEmpty(event.target.checked)} />
            允许空 Referer
          </label>
          <Tag type={allowEmpty ? "warning" : "danger"}>{allowEmpty ? "允许" : "拒绝"}</Tag>
          <p className="text-xs text-[#909399] mt-3 leading-relaxed">生产建议按业务场景关闭空 Referer，仅对 App、邮件或白名单下载链路发放签名 URL。</p>
          <Button type="primary" className="w-full mt-5" icon={ShieldCheck} disabled={saving} onClick={() => void saveHotlink()}>{saving ? "保存中..." : "保存并实时生效"}</Button>
          <div className="text-[10px] text-[#909399] mt-2">最近更新：{config?.updated_at ? String(config.updated_at).replace("T", " ").slice(0, 19) : "-"}</div>
        </div>
      </div>
    </Card>
  );
};

const SystemSettingsView = () => {
  const [config, setConfig] = useState<AdminSystemConfig | null>(null);
  const [notice, setNotice] = useState<Notice | null>(null);
  const [purgeOpen, setPurgeOpen] = useState(false);
  const [purgeConfirm, setPurgeConfirm] = useState("");
  const [purging, setPurging] = useState(false);
  useEffect(() => {
    adminFetch<AdminSystemConfig>("/admin/system/config")
      .then((data) => {
        setNotice(null);
        setConfig(data);
      })
      .catch((error) => {
        setConfig(null);
        setNotice({ type: "danger", text: error instanceof Error ? error.message : "系统配置读取失败，请检查 API 服务。" });
      });
  }, []);
  const purgeExpired = async () => {
    if (purging || purgeConfirm !== "PURGE") return;
    setPurging(true);
    try {
      const result = await adminFetch<{ purged_users: string[]; deleted_objects: number }>("/admin/jobs/purge-expired", { method: "POST" });
      setNotice({ type: "success", text: `超期资源清理完成：${result.purged_users.length} 个用户，${result.deleted_objects} 个对象。` });
      setPurgeOpen(false);
      setPurgeConfirm("");
    } catch (error) {
      setNotice({ type: "danger", text: error instanceof Error ? error.message : "超期资源清理失败，请检查后台服务。" });
    } finally {
      setPurging(false);
    }
  };
  const items = [
    ["环境", config?.app_env || "-"],
    ["Public URL", config?.public_base_url || "-"],
    ["Image URL", config?.image_public_base_url || "-"],
    ["存储驱动", config?.storage_driver || "-"],
    ["队列驱动", config?.queue_driver || "-"],
    ["Redis", config?.redis_addr || "-"],
    ["数据库", config?.database_enabled ? "enabled" : "disabled"],
    ["SMTP", config?.smtp_configured ? "configured" : "not configured"],
    ["IF-Pay", config?.ifpay_configured ? "configured" : "not configured"],
    ["违规 pHash 库", `${String(config?.moderation_hash_count ?? 0)} 条`],
  ];
  return (
    <>
      <Card title="全局系统设置" extra={<Tag type={config?.app_env === "production" ? "danger" : "warning"}>{config?.app_env || "unknown"}</Tag>}>
        <NoticeBar notice={notice} className="mb-4" />
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {items.map(([label, value]) => (
            <div key={label} className="border border-[#ebeef5] rounded p-4 bg-[#f5f7fa]">
              <div className="text-xs text-[#909399] mb-1">{label}</div>
              <div className="font-mono text-sm text-[#303133] break-all">{String(value)}</div>
            </div>
          ))}
        </div>
        <div className="mt-4 p-3 bg-[#fdf6ec] border border-[#faecd8] rounded text-xs text-[#e6a23c]">
          生产环境会强制校验管理员 Token、JWT Secret、图片签名 Secret、PostgreSQL、S3 与 CORS 白名单，未配置会拒绝启动。
        </div>
      </Card>
      <Card title="生命周期维护" extra={<Tag type="warning">高危任务</Tag>} className="mt-5">
        <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4">
          <div>
            <div className="text-sm font-semibold text-[#303133]">清理已超过保留期的订阅资源</div>
            <div className="text-xs text-[#909399] mt-1">调用后端清理任务，删除已进入 `deleted` 生命周期的对象并写入审计日志。</div>
          </div>
          <Button type="danger" icon={Trash2} onClick={() => setPurgeOpen(true)}>清理超期资源</Button>
        </div>
      </Card>
      <Dialog
        visible={purgeOpen}
        title="清理超期资源确认"
        danger
        confirmText={purging ? "清理中..." : "确认清理"}
        confirmDisabled={purging || purgeConfirm !== "PURGE"}
        onClose={() => { if (!purging) { setPurgeOpen(false); setPurgeConfirm(""); } }}
        onConfirm={() => void purgeExpired()}
      >
        <div className="space-y-4">
          <p>该操作会执行后台生命周期清理任务。对象文件删除后无法通过控制台恢复。</p>
          <FormItem label="确认口令" required helpText="请输入 PURGE 才能执行。">
            <Input value={purgeConfirm} onChange={setPurgeConfirm} placeholder="PURGE" />
          </FormItem>
        </div>
      </Dialog>
    </>
  );
};

const QueueStatusView = () => {
  const [status, setStatus] = useState<Row | null>(null);
  const [deadLetters, setDeadLetters] = useState<Row[]>([]);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState<{ type: ThemeTag; text: string } | null>(null);
  const [requeueID, setRequeueID] = useState("");
  const loadStatus = () => {
    setError("");
    adminFetch<Row>("/admin/queue/status")
      .then((data) => {
        setStatus(data);
        if (data.driver !== "redis") {
          setDeadLetters([]);
          return;
        }
        adminFetch<{ messages: Row[] }>("/admin/queue/dead-letters?limit=10")
          .then((letters) => setDeadLetters(letters.messages || []))
          .catch((err) => {
            setDeadLetters([]);
            setNotice({ type: "danger", text: err instanceof Error ? err.message : "Dead Letter 读取失败" });
          });
      })
      .catch((err) => {
        setStatus(null);
        setDeadLetters([]);
        setError(err instanceof Error ? err.message : "队列状态读取失败");
      });
  };
  useEffect(loadStatus, []);
  const requeueDeadLetter = async (id: string) => {
    if (requeueID) return;
    setRequeueID(id);
    try {
      await adminFetch(`/admin/queue/dead-letters/${encodeURIComponent(id)}/requeue`, { method: "POST" });
      setNotice({ type: "success", text: `Dead Letter ${id} 已重新投递。` });
      loadStatus();
    } catch (error) {
      setNotice({ type: "danger", text: error instanceof Error ? error.message : "重新投递失败，请检查队列服务。" });
    } finally {
      setRequeueID("");
    }
  };
  const consumers = (status?.consumers as Array<{ name: string; pending: number; idle_ms: number; inactive_ms: number }> | undefined) || [];
  const cards = [
    ["主队列长度", status?.length ?? 0, status?.reachable ? "primary" : "info"],
    ["Pending", status?.pending ?? 0, Number(status?.pending || 0) > 0 ? "warning" : "success"],
    ["Lag", status?.lag ?? 0, Number(status?.lag || 0) > 0 ? "warning" : "success"],
    ["Dead Letter", status?.dead_letter_length ?? 0, Number(status?.dead_letter_length || 0) > 0 ? "danger" : "success"],
  ] as const;
  return (
    <div className="space-y-4">
      {error && <div className="bg-[#fef0f0] border border-[#fde2e2] text-[#f56c6c] px-4 py-3 rounded text-sm">{error}</div>}
      {notice && <div className={`border px-4 py-3 rounded text-sm ${notice.type === "success" ? "bg-[#f0f9eb] border-[#e1f3d8] text-[#67c23a]" : "bg-[#fef0f0] border-[#fde2e2] text-[#f56c6c]"}`}>{notice.text}</div>}
      <Card
        title="Redis Stream 任务队列"
        extra={
          <div className="flex items-center gap-2">
            <Tag type={status?.reachable ? "success" : "danger"}>{status?.reachable ? "Redis 可达" : "Redis 不可达"}</Tag>
            <Button size="small" icon={RefreshCw} onClick={loadStatus}>刷新</Button>
          </div>
        }
      >
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-5">
          {cards.map(([label, value, type]) => (
            <div key={label} className="border border-[#ebeef5] rounded p-4 bg-[#f5f7fa]">
              <div className="text-xs text-[#909399] mb-1">{label}</div>
              <div className="text-2xl font-bold font-mono text-[#303133]">{String(value)}</div>
              <div className="mt-2"><Tag type={type}>{label === "Dead Letter" && Number(value) > 0 ? "需要处理" : "监控中"}</Tag></div>
            </div>
          ))}
        </div>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 text-sm">
          {[
            ["Driver", status?.driver || "-"],
            ["Stream", status?.stream || "-"],
            ["Dead Letter Stream", status?.dead_letter_stream || "-"],
            ["Consumer Group", status?.group || "-"],
            ["Worker", status?.worker_name || "-"],
            ["Retry Limit", status?.worker_retry_limit ?? "-"],
            ["Claim Idle", status?.worker_claim_idle || "-"],
            ["Last Generated ID", status?.last_generated_id || "-"],
          ].map(([label, value]) => (
            <div key={label} className="border border-[#ebeef5] rounded p-3 bg-white">
              <div className="text-xs text-[#909399] mb-1">{label}</div>
              <div className="font-mono text-[#303133] break-all">{String(value)}</div>
            </div>
          ))}
        </div>
        {status?.error && <div className="mt-4 bg-[#fdf6ec] border border-[#faecd8] text-[#e6a23c] px-4 py-3 rounded text-xs">{String(status.error)}</div>}
      </Card>
      <Card title="消费者实例" extra={<Tag type="info">{consumers.length} 个 Consumer</Tag>}>
        <Table
          columns={[
            { title: "Consumer", dataIndex: "name", render: (val) => <span className="font-mono text-xs">{val}</span> },
            { title: "Pending", dataIndex: "pending", render: (val) => <Tag type={Number(val) > 0 ? "warning" : "success"}>{val}</Tag> },
            { title: "Idle", dataIndex: "idle", render: (val) => <span className="font-mono text-xs">{val}</span> },
            { title: "Inactive", dataIndex: "inactive", render: (val) => <span className="font-mono text-xs">{val}</span> },
          ]}
          data={consumers.map((consumer) => ({
            name: consumer.name,
            pending: consumer.pending,
            idle: formatMS(consumer.idle_ms),
            inactive: formatMS(consumer.inactive_ms),
          }))}
        />
        {!consumers.length && <div className="text-xs text-[#909399] mt-3">尚未发现消费者。若生产环境出现此状态，请检查 `yuexiang-worker` 是否运行。</div>}
      </Card>
      <Card title="Dead Letter 任务" extra={<Tag type={deadLetters.length ? "danger" : "success"}>{deadLetters.length ? "需要排查" : "干净"}</Tag>}>
        <Table
          columns={[
            { title: "Dead ID", dataIndex: "id", render: (val) => <span className="font-mono text-xs">{val}</span> },
            { title: "类型", dataIndex: "type", render: (val) => <Tag type={val === "image.process" ? "primary" : "warning"}>{val}</Tag> },
            { title: "原始 ID", dataIndex: "original_id", render: (val) => <span className="font-mono text-xs">{val}</span> },
            { title: "失败次数", dataIndex: "delivery_count", render: (val) => <Tag type="danger">{val}</Tag> },
            { title: "失败原因", dataIndex: "reason", render: (val) => <span className="text-xs truncate max-w-xs block" title={String(val)}>{val}</span> },
            { title: "失败时间", dataIndex: "failed_at", render: (val) => <span className="text-xs">{String(val).replace("T", " ").slice(0, 19)}</span> },
            { title: "操作", dataIndex: "id", render: (val) => <Button type="text" size="small" disabled={Boolean(requeueID)} onClick={() => void requeueDeadLetter(String(val))}>{requeueID === val ? "投递中" : "重新投递"}</Button> },
          ]}
          data={deadLetters}
        />
        {!deadLetters.length && <div className="text-xs text-[#909399] mt-3">暂无 dead-letter。这个安静很好，说明坏任务没有堆起来。</div>}
      </Card>
    </div>
  );
};

const CDNConfigView = () => {
  const [config, setConfig] = useState<Row | null>(null);
  const [notice, setNotice] = useState("");
  useEffect(() => {
    adminFetch<Row>("/admin/cdn/config")
      .then((data) => {
        setNotice("");
        setConfig(data);
      })
      .catch((error) => {
        setConfig(null);
        setNotice(error instanceof Error ? error.message : "CDN 配置读取失败，请检查 API 服务。");
      });
  }, []);
  return (
    <Card title="CDN 分发网络" extra={<Tag type="primary">Edge Auth</Tag>}>
      {notice && <div className="mb-4 bg-[#fef0f0] border border-[#fde2e2] text-[#f56c6c] px-4 py-3 rounded text-sm">{notice}</div>}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {[
          ["图片域名", config?.image_public_base_url || "-"],
          ["原站鉴权", config?.origin_auth_endpoint || "-"],
          ["原图路由", config?.image_route || "-"],
          ["派生图路由", config?.variant_route || "-"],
          ["Nginx 配置", config?.nginx_config || "-"],
          ["Cloudflare 规则", config?.cloudflare_rules_doc || "-"],
        ].map(([label, value]) => (
          <div key={label} className="border border-[#ebeef5] rounded p-4 bg-[#f5f7fa]">
            <div className="text-xs text-[#909399] mb-1">{label}</div>
            <div className="font-mono text-sm text-[#303133] break-all">{String(value)}</div>
          </div>
        ))}
      </div>
      <div className="mt-4 text-xs text-[#606266] bg-[#ecf5ff] border border-[#d9ecff] rounded p-3">{String(config?.cache_strategy || "公开图可缓存，私有签名图按 query token 区分缓存。")}</div>
    </Card>
  );
};

const OpenPlatformView = () => {
  const [apiConfig, setAPIConfig] = useState<Row | null>(null);
  const [integration, setIntegration] = useState<IFPayIntegrationConfig | null>(null);
  const [form, setForm] = useState({
    ifpay_base_url: "",
    ifpay_partner_app_id: "",
    ifpay_client_id: "",
    ifpay_redirect_uri: "",
    ifpay_client_secret: "",
    ifpay_private_key_pem: "",
    ifpay_public_key_pem: "",
    ifpay_webhook_public_key_pem: "",
  });
  const [notice, setNotice] = useState<Notice | null>(null);
  const [saving, setSaving] = useState(false);

  const applyIntegration = (data: IFPayIntegrationConfig) => {
    setIntegration(data);
    setForm((current) => ({
      ...current,
      ifpay_base_url: data.ifpay_base_url || "",
      ifpay_partner_app_id: data.ifpay_partner_app_id || "",
      ifpay_client_id: data.ifpay_client_id || "",
      ifpay_redirect_uri: data.ifpay_redirect_uri || "",
      ifpay_client_secret: "",
      ifpay_private_key_pem: "",
      ifpay_public_key_pem: "",
      ifpay_webhook_public_key_pem: "",
    }));
  };

  const loadConfig = () => {
    Promise.all([
      adminFetch<Row>("/admin/api/config"),
      adminFetch<IFPayIntegrationConfig>("/admin/integrations/ifpay"),
    ])
      .then(([apiData, integrationData]) => {
        setAPIConfig(apiData);
        applyIntegration(integrationData);
        setNotice(null);
      })
      .catch((error) => {
        setAPIConfig(null);
        setIntegration(null);
        setNotice({ type: "danger", text: error instanceof Error ? error.message : "开放平台配置读取失败，请检查 API 服务。" });
      });
  };

  useEffect(loadConfig, []);

  const setField = (key: keyof typeof form, value: string) => setForm((current) => ({ ...current, [key]: value }));

  const saveIntegration = async () => {
    setSaving(true);
    try {
      const payload = {
        ifpay_base_url: form.ifpay_base_url.trim(),
        ifpay_partner_app_id: form.ifpay_partner_app_id.trim(),
        ifpay_client_id: form.ifpay_client_id.trim(),
        ifpay_redirect_uri: form.ifpay_redirect_uri.trim(),
        ifpay_client_secret: form.ifpay_client_secret.trim(),
        ifpay_private_key_pem: form.ifpay_private_key_pem.trim(),
        ifpay_public_key_pem: form.ifpay_public_key_pem.trim(),
        ifpay_webhook_public_key_pem: form.ifpay_webhook_public_key_pem.trim(),
      };
      const next = await adminFetch<IFPayIntegrationConfig>("/admin/integrations/ifpay", {
        method: "PATCH",
        body: JSON.stringify(payload),
      });
      applyIntegration(next);
      setNotice({ type: "success", text: "IF-Pay 支付与 OAuth 配置已保存，前台授权、下单、Webhook 会立即使用新配置。" });
    } catch (error) {
      setNotice({ type: "danger", text: error instanceof Error ? error.message : "保存失败，请检查后台服务。" });
    } finally {
      setSaving(false);
    }
  };

  const StatusCard = ({ title, ready, desc }: { title: string; ready: boolean; desc: string }) => (
    <div className={`rounded border p-4 ${ready ? "bg-[#f0f9eb] border-[#e1f3d8]" : "bg-[#fdf6ec] border-[#faecd8]"}`}>
      <div className={`flex items-center gap-2 text-sm font-semibold ${ready ? "text-[#67c23a]" : "text-[#e6a23c]"}`}>
        {ready ? <CheckCircle2 className="w-4 h-4" /> : <AlertCircle className="w-4 h-4" />}
        {title}
      </div>
      <div className="text-xs text-[#606266] mt-2 leading-relaxed">{desc}</div>
    </div>
  );

  const secretFields: Array<[keyof typeof form, string, boolean, string]> = [
    ["ifpay_client_secret", "OAuth Client Secret", Boolean(integration?.ifpay_client_secret_configured), "留空保持不变，填写后用于 code 换取 access token。"],
    ["ifpay_private_key_pem", "支付请求私钥 PEM", Boolean(integration?.ifpay_private_key_configured), "留空保持不变，填写后用于创建支付单 RSA 签名。"],
    ["ifpay_public_key_pem", "平台回传公钥 PEM", Boolean(integration?.ifpay_public_key_configured), "留空保持不变，保留给回跳或对账验签使用。"],
    ["ifpay_webhook_public_key_pem", "Webhook 公钥 PEM", Boolean(integration?.ifpay_webhook_public_key_configured), "留空保持不变，填写后 Webhook 强制验签。"],
  ];

  return (
    <div className="space-y-6">
      <NoticeBar notice={notice} />
      <Card title="API 与开放平台" extra={<Tag type="success">OpenAPI</Tag>}>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {[
            ["API Base", apiConfig?.public_base_url || "-"],
            ["OpenAPI", apiConfig?.openapi || "-"],
            ["Metrics", apiConfig?.metrics || "-"],
            ["IF-Pay OAuth", integration?.ifpay_oauth_start || apiConfig?.ifpay_oauth_start || "-"],
            ["IF-Pay Webhook", integration?.ifpay_webhook || apiConfig?.ifpay_webhook || "-"],
          ].map(([label, value]) => (
            <div key={label} className="border border-[#ebeef5] rounded p-4 bg-[#f5f7fa]">
              <div className="text-xs text-[#909399] mb-1">{label}</div>
              <div className="font-mono text-sm text-[#303133] break-all">{String(value)}</div>
            </div>
          ))}
        </div>
        <div className="mt-4">
          <div className="text-sm font-semibold text-[#303133] mb-2">默认 API Key Scope</div>
          <div className="flex flex-wrap gap-2">{((apiConfig?.default_api_scopes as string[]) || []).map((scope) => <Tag key={scope} type="primary">{scope}</Tag>)}</div>
        </div>
      </Card>

      <Card title="IF-Pay 支付与 OAuth 接入" extra={<Tag type={integration?.ifpay_configured ? "success" : "warning"}>{integration?.ifpay_configured ? "已配置" : "待配置"}</Tag>}>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
          <StatusCard
            title="OAuth 登录授权"
            ready={Boolean(integration?.ifpay_configured && integration?.ifpay_client_secret_configured)}
            desc="需要 Base URL、Client ID、Redirect URI 与 Client Secret。前台 IF-Pay 授权入口会读取这里的配置。"
          />
          <StatusCard
            title="支付单签名"
            ready={Boolean(integration?.ifpay_payment_signing_configured)}
            desc="需要支付请求私钥 PEM。创建订单后，后端会用该私钥向 IF-Pay 创建支付单。"
          />
          <StatusCard
            title="Webhook 验签"
            ready={Boolean(integration?.ifpay_webhook_verification_configured)}
            desc="配置 Webhook 公钥后，支付回调会校验 Digest 与 RSA-SHA256 签名再改订单状态。"
          />
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-x-6">
          <FormItem label="IF-Pay Base URL" required helpText="生产环境必须使用 HTTPS；本地联调允许 http://localhost 或 http://127.0.0.1。">
            <Input value={form.ifpay_base_url} onChange={(value) => setField("ifpay_base_url", value)} placeholder="https://pay.example.com" />
          </FormItem>
          <FormItem label="Partner App ID" required helpText="支付请求签名头使用的应用标识。">
            <Input value={form.ifpay_partner_app_id} onChange={(value) => setField("ifpay_partner_app_id", value)} placeholder="yuexiang-image" />
          </FormItem>
          <FormItem label="OAuth Client ID" required>
            <Input value={form.ifpay_client_id} onChange={(value) => setField("ifpay_client_id", value)} placeholder="ifpay_client_xxx" />
          </FormItem>
          <FormItem label="Redirect URI" required helpText="需与 IF-Pay 后台登记值完全一致。">
            <Input value={form.ifpay_redirect_uri} onChange={(value) => setField("ifpay_redirect_uri", value)} placeholder={`${window.location.origin}/oauth/ifpay/callback`} />
          </FormItem>
        </div>

        <div className="border-t border-[#ebeef5] pt-5 mt-1">
          <div className="text-sm font-semibold text-[#303133] mb-4">密钥材料</div>
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-x-6">
            {secretFields.map(([key, label, configured, helpText]) => (
              <FormItem key={key} label={label} helpText={`${configured ? "当前状态：已配置。" : "当前状态：未配置。"}${helpText}`}>
                <textarea
                  value={form[key]}
                  onChange={(event) => setField(key, event.target.value)}
                  className="w-full min-h-[96px] bg-white border border-[#dcdfe6] text-[#606266] rounded px-3 py-2 text-xs font-mono transition-colors focus:outline-none focus:border-[#409eff] placeholder-[#c0c4cc]"
                  placeholder={configured ? "留空保持不变；粘贴新密钥后替换" : "粘贴 PEM / Secret"}
                />
              </FormItem>
            ))}
          </div>
        </div>

        <div className="flex justify-end gap-3 border-t border-[#ebeef5] pt-4">
          <Button icon={RefreshCw} onClick={loadConfig}>重新读取</Button>
          <Button type="primary" icon={Check} disabled={saving} onClick={() => void saveIntegration()}>{saving ? "保存中" : "保存配置"}</Button>
        </div>
      </Card>
    </div>
  );
};

const AuditLogView = () => {
  const [logs, setLogs] = useState<Row[]>([]);
  const [notice, setNotice] = useState("");
  const [keyword, setKeyword] = useState("");
  const [actionFilter, setActionFilter] = useState("");
  const [total, setTotal] = useState(0);
  const loadLogs = () => {
    const params = new URLSearchParams({ limit: "100" });
    if (keyword.trim()) params.set("q", keyword.trim());
    if (actionFilter) params.set("action", actionFilter);
    adminFetch<{ items: Array<{ id: string; actor: string; action: string; target: string; metadata?: Record<string, unknown>; created_at: string }>; total: number }>(`/admin/audit-logs?${params.toString()}`)
      .then((payload) => {
        setNotice("");
        setTotal(payload.total);
        setLogs(payload.items.map((log) => ({
          id: log.id,
          time: log.created_at?.replace("T", " ").slice(0, 19) || "-",
          admin: log.actor,
          action: log.action,
          target: log.target,
          detail: log.metadata ? JSON.stringify(log.metadata) : "-",
          ip: "-",
        })));
      })
      .catch((error) => {
        setLogs([]);
        setTotal(0);
        setNotice(error instanceof Error ? error.message : "审计日志读取失败，请检查 API 服务。");
      });
  };
  useEffect(loadLogs, []);
  return (
    <Card title="系统安全审计日志" extra={<Tag type="info">命中 {total} 条</Tag>}>
      {notice && <div className="mb-4 bg-[#fef0f0] border border-[#fde2e2] text-[#f56c6c] px-4 py-3 rounded text-sm">{notice}</div>}
      <div className="flex gap-4 mb-4">
        <Input value={keyword} onChange={setKeyword} placeholder="操作员/目标ID/元数据" prefix={Search} className="w-64" />
        <select value={actionFilter} onChange={(event) => setActionFilter(event.target.value)} className="border border-[#dcdfe6] rounded px-3 py-2 text-sm bg-white">
          <option value="">全部操作类型</option>
          <option value="order.">订单与订阅</option>
          <option value="plan.">套餐权益</option>
          <option value="user.">用户状态</option>
          <option value="image.">图片治理</option>
          <option value="security.">安全配置</option>
        </select>
        <Button icon={Search} type="primary" onClick={loadLogs}>搜索</Button>
      </div>
      <Table
      columns={[
        { title: "发生时间", dataIndex: "time", render: (val) => <span className="text-xs">{val}</span> },
        { title: "操作员", dataIndex: "admin", render: (val) => <span className="font-medium text-[#303133]">{val}</span> },
        { title: "操作类型", dataIndex: "action", render: (val) => <Tag type={String(val).includes("封禁") || String(val).includes("冻结") ? "danger" : String(val).includes("发放") ? "warning" : "info"}>{val}</Tag> },
        { title: "目标对象", dataIndex: "target", render: (val) => <span className="font-mono text-xs">{val}</span> },
        { title: "详细日志", dataIndex: "detail", render: (val) => <span className="text-xs text-[#606266] truncate max-w-md block" title={String(val)}>{val}</span> },
        { title: "来源 IP", dataIndex: "ip", render: (val) => <span className="font-mono text-xs text-[#909399]">{val}</span> },
      ]}
        data={logs}
      />
    </Card>
  );
};

const MENU_ITEMS: MenuGroup[] = [
  { group: "运营中心", items: [
    { id: "dashboard", icon: LayoutDashboard, label: "后台总览" },
    { id: "users", icon: Users, label: "用户管理" },
    { id: "orders", icon: CreditCard, label: "订单与订阅" },
  ] },
  { group: "套餐与权益", items: [
    { id: "plans", icon: Zap, label: "公开套餐管理" },
    { id: "hidden_plans", icon: AlertTriangle, label: "隐藏套餐 (Max)" },
    { id: "invite_form", icon: LinkIcon, label: "创建邀请活动" },
    { id: "invite_records", icon: FileText, label: "邀请兑换记录" },
  ] },
  { group: "安全与风控", items: [
    { id: "moderation", icon: ShieldCheck, label: "图片内容审核" },
    { id: "security", icon: ShieldAlert, label: "安全风控 WAF" },
    { id: "hotlink", icon: Globe, label: "防盗链配置" },
  ] },
  { group: "系统基础设施", items: [
    { id: "storage", icon: HardDrive, label: "对象存储配置" },
    { id: "cdn", icon: Zap, label: "CDN 边缘节点" },
    { id: "api", icon: Key, label: "API 接口管理" },
    { id: "queue", icon: Activity, label: "任务队列状态" },
  ] },
  { group: "系统管理", items: [
    { id: "settings", icon: Settings, label: "全局设置" },
    { id: "backup", icon: Database, label: "备份与灾备恢复" },
    { id: "audit", icon: Activity, label: "系统审计日志" },
  ] },
];

const AdminAuthGate = ({ onAuthenticated }: { onAuthenticated: (admin: AdminAccount, token: string) => void }) => {
  const [loading, setLoading] = useState(true);
  const [setupRequired, setSetupRequired] = useState(false);
  const [notice, setNotice] = useState("");
  const [email, setEmail] = useState("admin@yuexiang.local");
  const [password, setPassword] = useState("");
  const [totpCode, setTOTPCode] = useState("");
  const [setup, setSetup] = useState<AdminBootstrapStart | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const authRequest = async <T,>(path: string, body?: unknown): Promise<T> => {
    const headers = new Headers();
    const token = getAdminSessionToken();
    if (token) {
      headers.set("Authorization", `Bearer ${token}`);
    }
    if (body) {
      headers.set("Content-Type", "application/json");
    }
    const res = await fetch(`${API_BASE}${path}`, {
      method: body ? "POST" : "GET",
      headers,
      body: body ? JSON.stringify(body) : undefined,
    });
    const payload = (await res.json()) as APIEnvelope<T>;
    if (!res.ok || !payload.ok) {
      throw new Error(payload.error?.message || `请求失败 (${res.status})`);
    }
    return payload.data;
  };

  useEffect(() => {
    let cancelled = false;
    authRequest<AdminAuthStatus>("/admin/auth/status")
      .then((status) => {
        if (cancelled) return;
        setSetupRequired(status.setup_required);
        if (status.admin) {
          onAuthenticated(status.admin, getAdminSessionToken());
        } else if (getAdminSessionToken()) {
          localStorage.removeItem(ADMIN_SESSION_KEY);
        }
      })
      .catch((error) => {
        if (!cancelled) setNotice(error instanceof Error ? error.message : "管理员状态读取失败");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const startBootstrap = async () => {
    if (submitting) return;
    try {
      setSubmitting(true);
      setNotice("");
      const result = await authRequest<AdminBootstrapStart>("/admin/auth/bootstrap/start", {
        email,
        display_name: email.split("@")[0],
        password,
      });
      setSetup(result);
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "管理员初始化失败");
    } finally {
      setSubmitting(false);
    }
  };

  const completeBootstrap = async () => {
    if (!setup || submitting) return;
    try {
      setSubmitting(true);
      setNotice("");
      const result = await authRequest<AdminAuthResult>("/admin/auth/bootstrap/complete", {
        setup_token: setup.setup_token,
        totp_code: totpCode,
      });
      localStorage.setItem(ADMIN_SESSION_KEY, result.token);
      onAuthenticated(result.admin, result.token);
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "两步验证失败");
    } finally {
      setSubmitting(false);
    }
  };

  const login = async () => {
    if (submitting) return;
    try {
      setSubmitting(true);
      setNotice("");
      const result = await authRequest<AdminAuthResult>("/admin/auth/login", {
        email,
        password,
        totp_code: totpCode,
      });
      localStorage.setItem(ADMIN_SESSION_KEY, result.token);
      onAuthenticated(result.admin, result.token);
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "管理员登录失败");
    } finally {
      setSubmitting(false);
    }
  };

  const submitCurrentStep = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (loading) return;
    if (setupRequired && !setup) {
      void startBootstrap();
      return;
    }
    if (setupRequired && setup) {
      void completeBootstrap();
      return;
    }
    void login();
  };

  return (
    <div className="min-h-screen bg-[#f2f6fc] flex items-center justify-center p-6">
      <div className="w-full max-w-sm bg-white border border-[#e4e7ed] rounded-md shadow-sm p-6">
        <div className="mb-6 text-center">
          <h1 className="text-xl font-semibold text-[#303133]">悦享图床 Admin</h1>
        </div>

        {loading ? (
          <div className="text-sm text-[#909399] py-8 text-center">加载中...</div>
        ) : setupRequired ? (
          <form onSubmit={submitCurrentStep}>
            <h2 className="text-base font-semibold text-[#303133] mb-4">{setup ? "绑定 2FA" : "首次初始化"}</h2>
            {!setup ? (
              <div className="space-y-4">
                <Input value={email} onChange={setEmail} placeholder="管理员邮箱" />
                <Input value={password} onChange={setPassword} type="password" placeholder="密码" />
                <Button type="primary" htmlType="submit" className="w-full" disabled={submitting}>{submitting ? "处理中..." : "生成二维码"}</Button>
              </div>
            ) : (
              <div className="space-y-4">
                <div className="flex justify-center">
                  <img src={setup.qr_code_data_url} alt="2FA QR Code" className="w-52 h-52 border border-[#ebeef5] rounded bg-white p-2" />
                </div>
                <Input value={totpCode} onChange={setTOTPCode} placeholder="6 位验证码" />
                <Button type="primary" htmlType="submit" className="w-full" icon={Check} disabled={submitting}>{submitting ? "验证中..." : "完成初始化"}</Button>
              </div>
            )}
          </form>
        ) : (
          <form onSubmit={submitCurrentStep}>
            <h2 className="text-base font-semibold text-[#303133] mb-4">管理员登录</h2>
            <div className="space-y-4">
              <Input value={email} onChange={setEmail} placeholder="管理员邮箱" />
              <Input value={password} onChange={setPassword} type="password" placeholder="密码" />
              <Input value={totpCode} onChange={setTOTPCode} placeholder="6 位验证码" />
              <Button type="primary" htmlType="submit" className="w-full" icon={Lock} disabled={submitting}>{submitting ? "登录中..." : "登录"}</Button>
            </div>
          </form>
        )}
        {notice && <div className="mt-4 bg-[#fef0f0] border border-[#fde2e2] text-[#f56c6c] px-3 py-2 rounded text-xs">{notice}</div>}
      </div>
    </div>
  );
};

function AdminConsole({ admin, onLogout }: { admin: AdminAccount; onLogout: () => void }) {
  const [currentRoute, setCurrentRoute] = useState<RouteID>("dashboard");
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const [apiStatus, setAPIStatus] = useState<"healthy" | "degraded" | "checking">("checking");

  useEffect(() => {
    injectGlobalStyles();
  }, []);

  useEffect(() => {
    let cancelled = false;
    fetch(`${API_ROOT}/readyz`)
      .then((res) => {
        if (!cancelled) setAPIStatus(res.ok ? "healthy" : "degraded");
      })
      .catch(() => {
        if (!cancelled) setAPIStatus("degraded");
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const renderView = () => {
    switch (currentRoute) {
      case "dashboard": return <DashboardView />;
      case "users": return <UserManageView />;
      case "orders": return <OrdersView />;
      case "plans": return <PlanManageView />;
      case "hidden_plans": return <HiddenPlanView />;
      case "invite_form": return <InviteCampaignView />;
      case "invite_records": return <InviteRecordsView />;
      case "moderation": return <ImageModView />;
      case "security": return <SecurityView />;
      case "hotlink": return <HotlinkConfigView />;
      case "storage": return <StorageConfigView />;
      case "backup": return <BackupView />;
      case "audit": return <AuditLogView />;
      case "cdn": return <CDNConfigView />;
      case "api": return <OpenPlatformView />;
      case "queue": return <QueueStatusView />;
      case "settings": return <SystemSettingsView />;
      default: return <DashboardView />;
    }
  };

  const getPageTitle = () => {
    for (const group of MENU_ITEMS) {
      const item = group.items.find((candidate) => candidate.id === currentRoute);
      if (item) return item.label;
    }
    return "管理控制台";
  };

  return (
    <div className="flex h-screen overflow-hidden bg-[#f2f6fc]">
      <aside className={`fixed inset-y-0 left-0 z-40 w-64 bg-[#2f3542] text-[#e4e7ed] transition-transform duration-300 ease-in-out lg:translate-x-0 lg:static lg:block ${isMobileMenuOpen ? "translate-x-0" : "-translate-x-full"}`}>
        <div className="flex items-center justify-center h-16 border-b border-[#1e272e] bg-[#222f3e]">
          <ImageIcon className="w-6 h-6 text-[#409eff] mr-2" />
          <span className="font-bold text-lg tracking-wider text-white">悦享图床 <span className="text-xs font-normal text-[#909399] ml-1">Admin</span></span>
        </div>
        <div className="overflow-y-auto h-[calc(100vh-4rem)] custom-scrollbar pb-10">
          {MENU_ITEMS.map((group) => (
            <div key={group.group} className="mb-4 mt-4">
              <div className="px-6 mb-2 text-xs font-semibold text-[#909399] uppercase tracking-wider">{group.group}</div>
              <nav className="space-y-1">
                {group.items.map((item) => {
                  const Icon = item.icon;
                  return (
                    <button
                      key={item.id}
                      type="button"
                      onClick={() => {
                        setCurrentRoute(item.id);
                        setIsMobileMenuOpen(false);
                      }}
                      className={`w-full flex items-center px-6 py-2.5 text-sm transition-colors ${
                        currentRoute === item.id
                          ? "bg-[#409eff]/20 text-[#409eff] border-r-4 border-[#409eff]"
                          : "text-[#a4b0be] hover:bg-[#1e272e] hover:text-white"
                      }`}
                    >
                      <Icon className={`w-4 h-4 mr-3 ${currentRoute === item.id ? "text-[#409eff]" : "text-[#a4b0be]"}`} />
                      {item.label}
                    </button>
                  );
                })}
              </nav>
            </div>
          ))}
        </div>
      </aside>

      {isMobileMenuOpen && <div className="fixed inset-0 z-30 bg-black/50 lg:hidden" onClick={() => setIsMobileMenuOpen(false)} />}

      <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
        <header className="h-16 bg-white border-b border-[#e4e7ed] flex items-center justify-between px-4 lg:px-6 z-10 shadow-sm">
          <div className="flex items-center gap-4">
            <button type="button" className="lg:hidden text-[#606266]" onClick={() => setIsMobileMenuOpen(true)}>
              <Menu className="w-6 h-6" />
            </button>
            <div className="flex items-center text-sm text-[#909399]">
              <span>运营中心</span>
              <ChevronRight className="w-4 h-4 mx-1" />
              <span className="font-semibold text-[#303133]">{getPageTitle()}</span>
            </div>
          </div>
          <div className="flex items-center gap-4">
            <div className="hidden md:flex items-center gap-2 text-xs text-[#909399]">
              <span className="flex items-center gap-1">
                <CheckCircle2 className={`w-3 h-3 ${apiStatus === "healthy" ? "text-[#67c23a]" : apiStatus === "degraded" ? "text-[#f56c6c]" : "text-[#e6a23c]"}`} />
                API {apiStatus === "healthy" ? "正常" : apiStatus === "degraded" ? "异常" : "检查中"}
              </span>
              <span className="w-px h-3 bg-[#dcdfe6] mx-1" />
              生产环境
            </div>
            <button
              type="button"
              className="flex items-center gap-2 hover:bg-[#f2f6fc] px-3 py-1.5 rounded transition-colors"
              onClick={onLogout}
            >
              <div className="w-7 h-7 rounded bg-[#409eff] text-white flex items-center justify-center text-xs font-bold shadow-sm">{admin.name.slice(0, 2).toUpperCase()}</div>
              <span className="text-sm font-medium text-[#606266] hidden sm:block">{admin.name}</span>
              <LogOut className="w-4 h-4 text-[#909399]" />
            </button>
          </div>
        </header>

        <main className="flex-1 overflow-auto p-4 lg:p-6 relative">
          <div className="max-w-[1600px] mx-auto">{renderView()}</div>
        </main>
      </div>
    </div>
  );
}

function YuexiangAdmin() {
  const [admin, setAdmin] = useState<AdminAccount | null>(null);

  useEffect(() => {
    injectGlobalStyles();
  }, []);

  useEffect(() => {
    const handleSessionExpired = () => setAdmin(null);
    const handleStorage = (event: StorageEvent) => {
      if (event.key === ADMIN_SESSION_KEY && !event.newValue) {
        setAdmin(null);
      }
    };
    window.addEventListener(ADMIN_SESSION_EXPIRED_EVENT, handleSessionExpired);
    window.addEventListener("storage", handleStorage);
    return () => {
      window.removeEventListener(ADMIN_SESSION_EXPIRED_EVENT, handleSessionExpired);
      window.removeEventListener("storage", handleStorage);
    };
  }, []);

  const handleAuthenticated = (nextAdmin: AdminAccount, token: string) => {
    if (token) {
      localStorage.setItem(ADMIN_SESSION_KEY, token);
    }
    setAdmin(nextAdmin);
  };

  const handleLogout = async () => {
    try {
      await fetch(`${API_BASE}/admin/auth/logout`, {
        method: "POST",
        headers: { Authorization: `Bearer ${getAdminSessionToken()}` },
      });
    } finally {
      clearAdminSession();
      setAdmin(null);
    }
  };

  if (!admin) {
    return <AdminAuthGate onAuthenticated={handleAuthenticated} />;
  }
  return <AdminConsole admin={admin} onLogout={() => void handleLogout()} />;
}

export default YuexiangAdmin;
export { YuexiangAdmin as App };
