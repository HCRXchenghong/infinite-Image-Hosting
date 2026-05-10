import React, { useEffect, useRef, useState } from "react";
import {
  AlertTriangle,
  Archive,
  Box,
  Check,
  ChevronRight,
  Copy,
  Cpu,
  Database,
  Download,
  FileJson,
  Filter,
  FolderOpen,
  Globe,
  Key,
  LayoutDashboard,
  Link as LinkIcon,
  Lock,
  LogOut,
  RefreshCw,
  Search,
  Settings2,
  ShieldAlert,
  ShieldCheck,
  TerminalSquare,
  Trash2,
  Unlock,
  Upload,
  X,
  Zap,
  type LucideIcon,
} from "lucide-react";

type ToastType = "success" | "error" | "info";
type TagType = "primary" | "success" | "warning" | "danger" | "info";
type DocsPanel = "openapi" | "markdown";
type View =
  | "landing"
  | "docs"
  | "auth"
  | "dashboard"
  | "upload"
  | "gallery"
  | "pricing"
  | "api"
  | "security"
  | "backup"
  | "settings";
type AppView = Exclude<View, "landing" | "docs" | "auth">;
type User = { id: string; name: string; email: string; plan: string; emailVerified: boolean; avatarURL?: string };
type ToastState = { msg: string; type: ToastType };
type RegisterResult = { userID: string; email: string; token: string; devCode?: string };
type ForgotPasswordResult = { email: string; devCode?: string };
type APIUsage = {
  storage_bytes?: number;
  bandwidth_bytes?: number;
  image_requests?: number;
  api_calls?: number;
  image_process_events?: number;
};
type ImagePrivacy = "public" | "private";
type ImageItem = {
  id: string;
  url: string;
  name: string;
  size: string;
  date: string;
  privacy: ImagePrivacy;
};
type PricingPlan = {
  id: string;
  name: string;
  priceMo: number;
  priceYr: number;
  storage: string;
  traffic: string;
  requests: string;
  api: string;
  process: string;
  color: string;
  popular?: boolean;
  storageBytes?: number | null;
  bandwidthBytes?: number | null;
  imageRequests?: number | null;
  apiCalls?: number | null;
  imageProcessEvents?: number | null;
};
type APIPlan = {
  slug: string;
  name: string;
  monthly_price_cent: number;
  yearly_price_cent: number;
  quota: {
    storage_bytes?: number | null;
    bandwidth_bytes?: number | null;
    image_requests?: number | null;
    api_calls?: number | null;
    image_process_events?: number | null;
  };
};
type ButtonVariant = "primary" | "secondary" | "danger" | "ghost";
type ButtonSize = "sm" | "default" | "lg";

const globalStyles = `
  @import url('https://fonts.googleapis.com/css2?family=Noto+Sans+SC:wght@400;500;700&display=swap');
  
  :root {
    --primary: #409EFF;
    --primary-hover: #66b1ff;
    --primary-light: #ecf5ff;
    --text-main: #303133;
    --text-regular: #606266;
    --text-secondary: #909399;
    --border-color: #dcdfe6;
    --bg-color: #f5f7fa;
  }

  body {
    font-family: 'Helvetica Neue', Helvetica, 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', '微软雅黑', Arial, sans-serif;
    background-color: var(--bg-color);
    color: var(--text-main);
    -webkit-font-smoothing: antialiased;
  }

  .panel-card {
    background: #ffffff;
    border: 1px solid var(--border-color);
    border-radius: 4px;
    box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.05);
    transition: box-shadow 0.3s;
  }
  
  .panel-card:hover {
    box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
  }

  .panel-header {
    background: #ffffff;
    border-bottom: 1px solid #ebeef5;
  }

  ::-webkit-scrollbar { width: 6px; height: 6px; }
  ::-webkit-scrollbar-track { background: transparent; }
  ::-webkit-scrollbar-thumb { background: #c0c4cc; border-radius: 3px; }
  ::-webkit-scrollbar-thumb:hover { background: #909399; }
`;

const API_BASE = import.meta.env.VITE_API_BASE_URL || "/api/v1";
const API_ROOT = API_BASE.replace(/\/api\/v1$/, "") || "";
const TOKEN_KEY = "yuexiang.session";
const IFPAY_TOKEN_KEY = "yuexiang.ifpay-access-token";

type APIEnvelope<T> = { ok: boolean; data: T; error?: { code: string; message: string } };
type APIUser = { id: string; email: string; nickname: string; avatar_url?: string; plan_slug: string; email_verified: boolean };
type APIImage = {
  public_id: string;
  filename: string;
  bytes: number;
  private: boolean;
  created_at: string;
};
type APIOrder = {
  id: string;
  plan_slug: string;
  billing_cycle: string;
  amount_cent: number;
  status: string;
  ifpay_payment_id: string;
  created_at: string;
  paid_at?: string;
  failed_at?: string;
  cancelled_at?: string;
  refunded_at?: string;
};

async function apiFetch<T>(path: string, options: RequestInit = {}, token?: string): Promise<T> {
  const headers = new Headers(options.headers);
  if (!(options.body instanceof FormData)) {
    headers.set("Content-Type", "application/json");
  }
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  const res = await fetch(`${API_BASE}${path}`, { ...options, headers });
  const payload = (await res.json()) as APIEnvelope<T>;
  if (!res.ok || !payload.ok) {
    throw new Error(payload.error?.message || `请求失败 (${res.status})`);
  }
  return payload.data;
}

const formatBytes = (value: number) => {
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  if (value < 1024 * 1024 * 1024) return `${(value / 1024 / 1024).toFixed(1)} MB`;
  return `${(value / 1024 / 1024 / 1024).toFixed(1)} GB`;
};
const formatQuotaBytes = (value?: number | null) => {
  if (value == null) return "不限量";
  if (value >= 1024 * 1024 * 1024 * 1024) return `${Number((value / 1024 / 1024 / 1024 / 1024).toFixed(1))}TB`;
  if (value >= 1024 * 1024 * 1024) return `${Math.round(value / 1024 / 1024 / 1024)}GB`;
  return formatBytes(value);
};
const formatQuotaCount = (value?: number | null) => {
  if (value == null) return "不限量";
  if (value >= 100_000_000) return `${Number((value / 100_000_000).toFixed(1))}亿`;
  if (value >= 10_000) return `${Math.round(value / 10_000)}万`;
  return String(value);
};
const meterPercent = (current = 0, max?: number | null) => {
  if (!max || max <= 0) return 0;
  return Math.min(100, Math.round((current / max) * 100));
};
const formatUsageCount = (value = 0) => formatQuotaCount(value);

const mapUser = (user: APIUser): User => ({
  id: user.id,
  name: user.nickname || user.email.split("@")[0],
  email: user.email,
  plan: user.plan_slug || "go",
  emailVerified: Boolean(user.email_verified),
  avatarURL: user.avatar_url || "",
});

const mapImage = (image: APIImage): ImageItem => ({
  id: image.public_id,
  url: `${API_BASE.replace(/\/api\/v1$/, "")}/i/${image.public_id}`,
  name: image.filename,
  size: formatBytes(image.bytes),
  date: image.created_at?.slice(0, 10) || new Date().toISOString().slice(0, 10),
  privacy: image.private ? "private" : "public",
});

const mapPlan = (plan: APIPlan): PricingPlan => ({
  id: plan.slug,
  name: plan.name,
  priceMo: Math.round(plan.monthly_price_cent / 100),
  priceYr: Math.round(plan.yearly_price_cent / 100),
  storage: formatQuotaBytes(plan.quota?.storage_bytes),
  traffic: formatQuotaBytes(plan.quota?.bandwidth_bytes),
  requests: formatQuotaCount(plan.quota?.image_requests),
  api: formatQuotaCount(plan.quota?.api_calls),
  process: formatQuotaCount(plan.quota?.image_process_events),
  color: plan.slug === "plus" ? "bg-blue-50 border-blue-200" : plan.slug === "ultra" ? "bg-slate-800 text-white" : "bg-slate-50 border-slate-200",
  popular: plan.slug === "plus",
  storageBytes: plan.quota?.storage_bytes,
  bandwidthBytes: plan.quota?.bandwidth_bytes,
  imageRequests: plan.quota?.image_requests,
  apiCalls: plan.quota?.api_calls,
  imageProcessEvents: plan.quota?.image_process_events,
});

const Button = ({
  children,
  variant = "primary",
  size = "default",
  className = "",
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
  size?: ButtonSize;
}) => {
  const base = "inline-flex items-center justify-center font-medium rounded transition-colors duration-200 active:scale-[0.98] disabled:opacity-50 disabled:pointer-events-none disabled:cursor-not-allowed";
  const sizes: Record<ButtonSize, string> = {
    sm: "px-3 py-1.5 text-xs",
    default: "px-4 py-2 text-sm",
    lg: "px-6 py-2.5 text-base",
  };
  const variants: Record<ButtonVariant, string> = {
    primary: "bg-[#409EFF] text-white hover:bg-[#66b1ff] border border-transparent",
    secondary: "bg-white text-[#606266] border border-[#dcdfe6] hover:text-[#409EFF] hover:border-[#c6e2ff] hover:bg-[#ecf5ff]",
    danger: "bg-[#f56c6c] text-white hover:bg-[#f78989] border border-transparent",
    ghost: "text-[#909399] hover:text-[#409EFF] hover:bg-transparent",
  };
  return (
    <button className={`${base} ${sizes[size]} ${variants[variant]} ${className}`} {...props}>
      {children}
    </button>
  );
};

const PanelCard = ({ children, className = "", ...props }: React.HTMLAttributes<HTMLDivElement>) => (
  <div className={`panel-card p-5 ${className}`} {...props}>
    {children}
  </div>
);

const Tag = ({ children, type = "info" }: { children: React.ReactNode; type?: TagType }) => {
  const styles: Record<TagType, string> = {
    primary: "bg-[#ecf5ff] text-[#409EFF] border-[#d9ecff]",
    success: "bg-[#f0f9eb] text-[#67c23a] border-[#e1f3d8]",
    warning: "bg-[#fdf6ec] text-[#e6a23c] border-[#faecd8]",
    danger: "bg-[#fef0f0] text-[#f56c6c] border-[#fde2e2]",
    info: "bg-[#f4f4f5] text-[#909399] border-[#e9e9eb]",
  };
  return <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs border ${styles[type]}`}>{children}</span>;
};

const UsageMeter = ({
  label,
  current,
  max,
  unit,
  percent,
  colorClass = "bg-[#409EFF]",
}: {
  label: string;
  current: string;
  max: string;
  unit: string;
  percent: number;
  colorClass?: string;
}) => (
  <div className="mb-4">
    <div className="flex justify-between text-xs mb-1.5">
      <span className="text-[#606266]">{label}</span>
      <span className="text-[#303133] font-medium">
        {current} <span className="text-[#909399] font-normal">/ {max} {unit}</span>
      </span>
    </div>
    <div className="w-full bg-[#ebeef5] rounded-sm h-1.5 overflow-hidden">
      <div className={`h-1.5 rounded-sm ${colorClass} transition-all duration-1000 ease-out`} style={{ width: `${Math.min(percent, 100)}%` }} />
    </div>
  </div>
);

let toastTimeout: ReturnType<typeof setTimeout> | undefined;
const Toast = ({ message, type = "info", onClose }: { message: string; type?: ToastType; onClose: () => void }) => {
  useEffect(() => {
    toastTimeout = setTimeout(onClose, 3000);
    return () => {
      if (toastTimeout) clearTimeout(toastTimeout);
    };
  }, [onClose]);

  const colors: Record<ToastType, string> = {
    success: "bg-[#f0f9eb] text-[#67c23a] border-[#e1f3d8]",
    error: "bg-[#fef0f0] text-[#f56c6c] border-[#fde2e2]",
    info: "bg-[#edf2fc] text-[#909399] border-[#ebeef5]",
  };

  return (
    <div className={`fixed top-6 left-1/2 -translate-x-1/2 px-4 py-2.5 rounded border shadow-sm flex items-center gap-2 z-50 ${colors[type]}`}>
      {type === "success" && <Check size={16} className="text-[#67c23a]" />}
      {type === "error" && <AlertTriangle size={16} className="text-[#f56c6c]" />}
      <span className="text-sm">{message}</span>
      <button onClick={onClose} className="ml-4 text-slate-400 hover:text-slate-600">
        <X size={14} />
      </button>
    </div>
  );
};

export function App() {
  const [currentView, setCurrentView] = useState<View>("landing");
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string>(() => localStorage.getItem(TOKEN_KEY) || "");
  const [ifpayToken, setIFPayToken] = useState<string>(() => localStorage.getItem(IFPAY_TOKEN_KEY) || "");
  const [toast, setToast] = useState<ToastState | null>(null);
  const [images, setImages] = useState<ImageItem[]>([]);
  const [plans, setPlans] = useState<PricingPlan[]>([]);
  const [usage, setUsage] = useState<APIUsage>({});
  const [plansError, setPlansError] = useState("");
  const [ordersRefreshKey, setOrdersRefreshKey] = useState(0);

  const showToast = (msg: string, type: ToastType = "info") => setToast({ msg, type });
  const openDocs = () => {
    window.history.pushState({}, "", `${window.location.pathname}${window.location.search}#/docs`);
    setCurrentView("docs");
    window.scrollTo({ top: 0 });
  };
  const closeDocs = () => {
    window.history.pushState({}, "", `${window.location.pathname}${window.location.search}`);
    setCurrentView("landing");
    window.scrollTo({ top: 0 });
  };

  useEffect(() => {
    const syncHashRoute = () => {
      if (window.location.hash === "#/docs") {
        setCurrentView("docs");
      }
    };
    syncHashRoute();
    window.addEventListener("hashchange", syncHashRoute);
    return () => window.removeEventListener("hashchange", syncHashRoute);
  }, []);

  useEffect(() => {
    if (!window.location.pathname.endsWith("/oauth/ifpay/callback")) return;
    const params = new URLSearchParams(window.location.search);
    const code = params.get("code");
    const state = params.get("state");
    if (!code) return;
    let cancelled = false;
    apiFetch<{ user: APIUser; token: string; ifpay_access_token: string }>(`/oauth/ifpay/callback?code=${encodeURIComponent(code)}&state=${encodeURIComponent(state || "")}`)
      .then((data) => {
        if (cancelled) return;
        localStorage.setItem(TOKEN_KEY, data.token);
        localStorage.setItem(IFPAY_TOKEN_KEY, data.ifpay_access_token);
        setToken(data.token);
        setIFPayToken(data.ifpay_access_token);
        setUser(mapUser(data.user));
        setCurrentView("pricing");
        window.history.replaceState({}, "", window.location.origin + window.location.pathname.replace(/\/oauth\/ifpay\/callback$/, "/"));
        showToast("IF-Pay 授权完成，可以继续订购资源包", "success");
      })
      .catch((error) => showToast(error instanceof Error ? error.message : "IF-Pay 授权失败", "error"));
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    apiFetch<APIPlan[]>("/plans")
      .then((rows) => {
        setPlans(rows.map(mapPlan));
        setPlansError("");
      })
      .catch((error) => {
        setPlans([]);
        setPlansError(error instanceof Error ? error.message : "套餐配置读取失败");
      });
  }, []);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    Promise.all([
      apiFetch<{ user: APIUser; usage: APIUsage }>("/auth/me", {}, token),
      apiFetch<APIImage[]>("/images", {}, token),
    ])
      .then(([me, imageList]) => {
        if (cancelled) return;
        setUser(mapUser(me.user));
        setUsage(me.usage || {});
        setImages(imageList.map(mapImage));
        setCurrentView((view) => (view === "landing" || view === "auth" ? "dashboard" : view));
      })
      .catch(() => {
        localStorage.removeItem(TOKEN_KEY);
        setToken("");
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  const handleLogin = async (email: string, password: string) => {
    const data = await apiFetch<{ user: APIUser; token: string }>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    });
    localStorage.setItem(TOKEN_KEY, data.token);
    setToken(data.token);
    setUser(mapUser(data.user));
    setUsage({});
    setCurrentView("dashboard");
    showToast("登录成功", "success");
  };

  const handleRegister = async (email: string, password: string, nickname: string): Promise<RegisterResult> => {
    const data = await apiFetch<{ user: APIUser; token: string; dev_email_verification_code?: string }>("/auth/register", {
      method: "POST",
      body: JSON.stringify({ email, password, nickname }),
    });
    showToast("注册成功，请输入邮箱验证码", "success");
    return {
      userID: data.user.id,
      email: data.user.email,
      token: data.token,
      devCode: data.dev_email_verification_code,
    };
  };

  const handleVerifyEmail = async (userID: string, code: string, nextToken: string) => {
    const verifiedUser = await apiFetch<APIUser>("/auth/verify-email", {
      method: "POST",
      body: JSON.stringify({ user_id: userID, code }),
    });
    localStorage.setItem(TOKEN_KEY, nextToken);
    setToken(nextToken);
    setUser(mapUser(verifiedUser));
    setUsage({});
    setCurrentView("dashboard");
    showToast("邮箱验证完成", "success");
  };

  const handleResendVerification = async (userID: string): Promise<string | undefined> => {
    const data = await apiFetch<{ message: string; dev_email_verification_code?: string }>("/auth/resend-verification", {
      method: "POST",
      body: JSON.stringify({ user_id: userID }),
    });
    showToast(data.message || "验证码已重新发送", "success");
    return data.dev_email_verification_code;
  };

  const handleForgotPassword = async (email: string): Promise<ForgotPasswordResult> => {
    const data = await apiFetch<{ message: string; dev_password_reset_code?: string }>("/auth/forgot-password", {
      method: "POST",
      body: JSON.stringify({ email }),
    });
    showToast(data.message || "如果邮箱存在，系统会发送重置验证码", "success");
    return { email, devCode: data.dev_password_reset_code };
  };

  const handleResetPassword = async (email: string, code: string, newPassword: string) => {
    await apiFetch<{ message: string }>("/auth/reset-password", {
      method: "POST",
      body: JSON.stringify({ email, code, new_password: newPassword }),
    });
    showToast("密码已重置，请重新登录", "success");
  };

  const startIFPayOAuth = async () => {
    const data = await apiFetch<{ authorization_url: string }>("/oauth/ifpay/start");
    window.location.assign(data.authorization_url);
  };

  const handleCheckout = async (planSlug: string, billingCycle: "monthly" | "yearly" = "monthly") => {
    if (!token) {
      setCurrentView("auth");
      showToast("请先登录后再订购资源包", "error");
      return;
    }
    if (!ifpayToken) {
      showToast("请先完成 IF-Pay 授权", "info");
      await startIFPayOAuth();
      return;
    }
    const data = await apiFetch<{ order: { id: string; status: string }; payment: { payment_id: string; status: string; qr_session_id?: string; review_id?: string } }>("/checkout/ifpay", {
      method: "POST",
      body: JSON.stringify({
        plan_slug: planSlug,
        billing_cycle: billingCycle,
        sub_method: "ifpay_balance",
        access_token: ifpayToken,
      }),
    }, token);
    showToast(`订单已创建：${data.order.id}，支付状态 ${data.payment.status}`, "success");
    setOrdersRefreshKey((current) => current + 1);
  };

  const handleUploadFile = async (file: File) => {
    if (!token) {
      throw new Error("请先登录");
    }
    const body = new FormData();
    body.append("file", file);
    const data = await apiFetch<{ image: APIImage; links: { raw: string } }>("/images", {
      method: "POST",
      body,
    }, token);
    return { ...mapImage(data.image), url: data.links.raw };
  };

  const refreshImages = async () => {
    if (!token) return;
    const imageList = await apiFetch<APIImage[]>("/images", {}, token);
    setImages(imageList.map(mapImage));
  };

  const refreshAccount = async () => {
    if (!token) return;
    const me = await apiFetch<{ user: APIUser; usage: APIUsage }>("/auth/me", {}, token);
    setUser(mapUser(me.user));
    setUsage(me.usage || {});
  };

  const handleDeleteImage = async (publicID: string) => {
    if (!token) throw new Error("请先登录");
    await apiFetch(`/images/${encodeURIComponent(publicID)}`, { method: "DELETE" }, token);
    setImages((current) => current.filter((image) => image.id !== publicID));
  };

  const handleTogglePrivacy = async (publicID: string, nextPrivacy: ImagePrivacy) => {
    if (!token) throw new Error("请先登录");
    const image = await apiFetch<APIImage>(`/images/${encodeURIComponent(publicID)}/privacy`, {
      method: "PATCH",
      body: JSON.stringify({ private: nextPrivacy === "private" }),
    }, token);
    const mapped = mapImage(image);
    setImages((current) => current.map((item) => (item.id === publicID ? mapped : item)));
    return mapped;
  };

  const handleSignImage = async (publicID: string) => {
    if (!token) throw new Error("请先登录");
    const data = await apiFetch<{ url: string; expires_at: number }>(`/images/${encodeURIComponent(publicID)}/sign`, {}, token);
    return data.url;
  };

  const handleLogout = () => {
    setUser(null);
    setUsage({});
    setToken("");
    setIFPayToken("");
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(IFPAY_TOKEN_KEY);
    setCurrentView("landing");
    showToast("已安全退出");
  };

  const navigate = (view: AppView) => {
    if (!user) {
      setCurrentView("auth");
      showToast("请先登录", "error");
      return;
    }
    setCurrentView(view);
  };

  const activeAppView: AppView = currentView === "landing" || currentView === "docs" || currentView === "auth" ? "dashboard" : currentView;

  return (
    <div className="min-h-screen relative overflow-x-hidden font-sans bg-[#f5f7fa]">
      <style dangerouslySetInnerHTML={{ __html: globalStyles }} />

      {toast && <Toast message={toast.msg} type={toast.type} onClose={() => setToast(null)} />}

      {currentView === "docs" ? (
        <DocsView
          onBack={closeDocs}
          onLogin={() => {
            window.history.pushState({}, "", `${window.location.pathname}${window.location.search}`);
            setCurrentView(user ? "dashboard" : "auth");
          }}
        />
      ) : !user ? (
        currentView === "auth" ? (
          <AuthView
            onLogin={handleLogin}
            onRegister={handleRegister}
            onVerifyEmail={handleVerifyEmail}
            onResendVerification={handleResendVerification}
            onForgotPassword={handleForgotPassword}
            onResetPassword={handleResetPassword}
            onStartIFPayOAuth={startIFPayOAuth}
            onBack={() => setCurrentView("landing")}
          />
        ) : (
          <LandingView onLogin={() => setCurrentView("auth")} onOpenDocs={openDocs} plans={plans} plansError={plansError} />
        )
      ) : (
        <AppLayout user={user} usage={usage} plans={plans} onLogout={handleLogout} currentView={activeAppView} onNavigate={navigate}>
          {activeAppView === "dashboard" && <DashboardView user={user} usage={usage} plans={plans} />}
          {activeAppView === "upload" && (
            <UploadView
              user={user}
              onUploadFile={handleUploadFile}
              onUploadSuccess={(img) => {
                setImages((current) => [img, ...current]);
                showToast("上传成功", "success");
                navigate("gallery");
                void refreshImages();
                void refreshAccount();
              }}
              onUploadError={(message) => showToast(message, "error")}
              onResendVerification={async () => {
                await handleResendVerification(user.id);
              }}
            />
          )}
          {activeAppView === "gallery" && (
            <GalleryView
              images={images}
              showToast={showToast}
              onDeleteImage={handleDeleteImage}
              onTogglePrivacy={handleTogglePrivacy}
              onSignImage={handleSignImage}
            />
          )}
          {activeAppView === "pricing" && (
            <PricingView
              token={token}
              plans={plans}
              plansError={plansError}
              currentPlanSlug={user.plan}
              ifpayConnected={Boolean(ifpayToken)}
              ordersRefreshKey={ordersRefreshKey}
              onConnectIFPay={startIFPayOAuth}
              onCheckout={handleCheckout}
            />
          )}
          {activeAppView === "api" && (
            <ApiView
              user={user}
              token={token}
              showToast={showToast}
              onResendVerification={async () => {
                await handleResendVerification(user.id);
              }}
            />
          )}
          {activeAppView === "security" && <SecurityView />}
          {activeAppView === "backup" && <BackupView token={token} showToast={showToast} />}
          {activeAppView === "settings" && (
            <SettingsView
              user={user}
              token={token}
              showToast={showToast}
              onUserUpdate={setUser}
              onResendVerification={async () => {
                await handleResendVerification(user.id);
              }}
            />
          )}
        </AppLayout>
      )}
    </div>
  );
}

const scrollToLandingSection = (id: string) => {
  document.getElementById(id)?.scrollIntoView({ behavior: "smooth", block: "start" });
};

const LandingView = ({ onLogin, onOpenDocs, plans, plansError }: { onLogin: () => void; onOpenDocs: () => void; plans: PricingPlan[]; plansError: string }) => (
  <div className="min-h-screen flex flex-col bg-white">
    <header className="panel-header sticky top-0 z-40 px-8 py-3 flex justify-between items-center">
      <div className="flex items-center gap-2">
        <Database className="text-[#409EFF]" size={24} strokeWidth={2} />
        <span className="text-xl font-bold text-[#303133] tracking-wide">悦享图床</span>
      </div>
      <nav className="hidden md:flex items-center gap-8 text-sm text-[#606266]">
        <button type="button" onClick={() => scrollToLandingSection("features")} className="hover:text-[#409EFF] transition-colors">架构优势</button>
        <button type="button" onClick={() => scrollToLandingSection("security")} className="hover:text-[#409EFF] transition-colors">安全控制</button>
        <button type="button" onClick={() => scrollToLandingSection("pricing")} className="hover:text-[#409EFF] transition-colors">订阅计费</button>
        <button type="button" onClick={onOpenDocs} className="hover:text-[#409EFF] transition-colors">开放 API</button>
      </nav>
      <div className="flex gap-3">
        <Button variant="ghost" onClick={onLogin}>登录控制台</Button>
        <Button onClick={onLogin}>立即开通</Button>
      </div>
    </header>

    <main className="flex-1">
      <section className="pt-24 pb-20 px-6 text-center max-w-5xl mx-auto">
        <div className="inline-flex items-center gap-2 px-3 py-1 rounded bg-[#ecf5ff] border border-[#d9ecff] text-[#409EFF] text-xs font-medium mb-8">
          <Cpu size={14} />
          Serverless 架构 稳定版已上线
        </div>
        <h1 className="text-5xl md:text-6xl font-bold text-[#303133] mb-6 leading-tight">
          高可用企业级 <br className="md:hidden" /><span className="text-[#409EFF]">对象存储与分发平台</span>
        </h1>
        <p className="text-base md:text-lg text-[#606266] mb-10 max-w-2xl mx-auto leading-relaxed">
          专为开发者构建的基础设施。提供低延迟全球 CDN 节点、严格的防盗链鉴权机制、高吞吐量的 OpenAPI 以及自动化的数据容灾备份方案。
        </p>
        <div className="flex flex-col sm:flex-row justify-center gap-4">
          <Button size="lg" className="px-8 shadow-sm" onClick={onLogin}>访问控制台 <ChevronRight className="ml-1" size={18} /></Button>
          <Button size="lg" variant="secondary" className="px-8" onClick={onOpenDocs}>阅读技术文档</Button>
        </div>

        <div className="mt-16 relative mx-auto max-w-4xl rounded border border-[#dcdfe6] shadow-lg overflow-hidden bg-white p-1">
          <div className="h-8 bg-[#f5f7fa] border-b border-[#dcdfe6] flex items-center px-3 gap-1.5 rounded-t-sm">
            <div className="w-2.5 h-2.5 rounded-full bg-[#f56c6c]" />
            <div className="w-2.5 h-2.5 rounded-full bg-[#e6a23c]" />
            <div className="w-2.5 h-2.5 rounded-full bg-[#67c23a]" />
          </div>
          <img src="https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?q=80&w=1200&auto=format&fit=crop" alt="Console Preview" className="w-full h-auto object-cover grayscale-[20%]" />
        </div>
      </section>

      <section id="features" className="scroll-mt-20 py-20 px-6 bg-white border-t border-[#ebeef5]">
        <div className="max-w-6xl mx-auto">
          <div className="text-center mb-12">
            <h2 className="text-2xl font-bold text-[#303133] mb-3">从上传到分发的完整基础设施</h2>
            <p className="text-[#606266] text-sm">对象存储、边缘处理、签名访问和任务队列都已串成真实业务链路。</p>
          </div>
          <div className="grid md:grid-cols-2 lg:grid-cols-4 gap-5">
            {[
              { icon: Database, title: "私有 Bucket 存储", desc: "原图与派生图进入对象存储，删除、冻结和备份都回写真实后端状态。" },
              { icon: Globe, title: "CDN 分发入口", desc: "统一通过 /i/{public_id} 分发，公开图可缓存，私有图走短期签名 URL。" },
              { icon: Zap, title: "异步图像处理", desc: "上传后投递处理任务，支持缩略图、WebP 与后续扩展的边缘处理额度。" },
              { icon: TerminalSquare, title: "开放 API 能力", desc: "前台、后台、API Key、备份、审计和 IF-Pay 支付路径统一走 OpenAPI。" },
            ].map((item) => {
              const Icon = item.icon;
              return (
                <div key={item.title} className="bg-[#f8fafc] border border-[#e4e7ed] rounded p-5 hover:border-[#409EFF] transition-colors">
                  <div className="w-10 h-10 rounded bg-[#ecf5ff] text-[#409EFF] flex items-center justify-center mb-4">
                    <Icon size={20} />
                  </div>
                  <h3 className="text-sm font-bold text-[#303133] mb-2">{item.title}</h3>
                  <p className="text-xs text-[#606266] leading-relaxed">{item.desc}</p>
                </div>
              );
            })}
          </div>
        </div>
      </section>

      <section id="security" className="scroll-mt-20 py-20 px-6 bg-[#f5f7fa] border-t border-[#ebeef5]">
        <div className="max-w-6xl mx-auto grid lg:grid-cols-3 gap-6 items-stretch">
          <div className="lg:col-span-1">
            <div className="inline-flex items-center gap-2 px-3 py-1 rounded bg-[#fef0f0] border border-[#fde2e2] text-[#f56c6c] text-xs font-medium mb-5">
              <ShieldAlert size={14} />
              风控策略已接入
            </div>
            <h2 className="text-2xl font-bold text-[#303133] mb-3">默认按生产安全模型设计</h2>
            <p className="text-sm text-[#606266] leading-relaxed">防盗链、私有读、短期签名、违规 pHash、冻结删除和审计日志不是页面装饰，后台都有对应接口和状态流转。</p>
          </div>
          <div className="lg:col-span-2 grid md:grid-cols-2 gap-4">
            {[
              ["Referer 防盗链", "后台可维护允许/阻断域名，图片分发入口实时应用。"],
              ["STS 签名 URL", "私有对象通过 15 分钟短链访问，避免长期裸链泄露。"],
              ["内容治理队列", "命中违规特征可冻结对象，并在管理端人工复核删除。"],
              ["安全审计日志", "套餐发放、订单对账、冻结删除、配置变更都会写入审计。"],
            ].map(([title, desc]) => (
              <div key={title} className="bg-white border border-[#e4e7ed] rounded p-5">
                <div className="text-sm font-bold text-[#303133] mb-2 flex items-center gap-2">
                  <ShieldCheck size={16} className="text-[#67c23a]" />
                  {title}
                </div>
                <p className="text-xs text-[#606266] leading-relaxed">{desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section id="pricing" className="scroll-mt-20 py-20 px-6 bg-[#f5f7fa] border-t border-[#ebeef5]">
        <div className="max-w-6xl mx-auto">
          <div className="text-center mb-12">
            <h2 className="text-2xl font-bold text-[#303133] mb-3">透明、可预测的计费模型</h2>
            <p className="text-[#606266] text-sm">按照您的实际业务规模选择合适的计算与存储配额。</p>
          </div>
          {plansError && <div className="mb-5 text-center text-xs text-[#f56c6c] bg-[#fef0f0] border border-[#fde2e2] rounded p-3">{plansError}</div>}
          {plans.length === 0 ? (
            <PanelCard className="text-center py-8 border-dashed text-sm text-[#909399]">套餐配置暂不可用，请稍后刷新或联系管理员。</PanelCard>
          ) : (
          <div className="grid md:grid-cols-2 lg:grid-cols-4 gap-5">
            {plans.map((plan) => (
              <div key={plan.id} className={`bg-white border rounded p-6 transition-all hover:-translate-y-1 hover:shadow-md ${plan.popular ? "border-[#409EFF] shadow-sm relative" : "border-[#dcdfe6]"}`}>
                {plan.popular && <div className="absolute top-0 right-0 bg-[#409EFF] text-white px-2 py-0.5 rounded-bl text-xs">推荐配置</div>}
                <h3 className="text-lg font-bold text-[#303133] mb-1">{plan.name}</h3>
                <div className="flex items-baseline gap-1 mb-6 border-b border-[#ebeef5] pb-4">
                  <span className="text-3xl font-bold text-[#409EFF]">¥{plan.priceMo}</span>
                  <span className="text-[#909399] text-sm">/ 月</span>
                </div>
                <ul className="space-y-3 mb-8 flex-1 text-sm text-[#606266]">
                  <li className="flex items-center gap-2"><Check size={16} className="text-[#67c23a]" /> {plan.storage} 分布式存储</li>
                  <li className="flex items-center gap-2"><Check size={16} className="text-[#67c23a]" /> {plan.traffic} 下行回源流量</li>
                  <li className="flex items-center gap-2"><Check size={16} className="text-[#67c23a]" /> {plan.requests} 次 HTTP 请求</li>
                  <li className="flex items-center gap-2"><Check size={16} className="text-[#67c23a]" /> {plan.api} 次 API 限额</li>
                  <li className="flex items-center gap-2"><Check size={16} className="text-[#67c23a]" /> {plan.process} 图像边缘处理</li>
                </ul>
                <Button variant={plan.popular ? "primary" : "secondary"} className="w-full" onClick={onLogin}>部署 {plan.name} 实例</Button>
              </div>
            ))}
          </div>
          )}
          <div className="mt-8 text-center text-xs text-[#909399] flex items-center justify-center gap-1.5">
            <AlertTriangle size={14} /> 实例到期后进入 30 天资源保留期，保留期内停止写入权限。超过 30 天将触发自动销毁脚本，数据不可恢复。
          </div>
        </div>
      </section>
    </main>
  </div>
);

const DocsView = ({ onBack, onLogin }: { onBack: () => void; onLogin: () => void }) => {
  const [activePanel, setActivePanel] = useState<DocsPanel>("openapi");
  const [openapiText, setOpenapiText] = useState("");
  const [markdownText, setMarkdownText] = useState("");
  const [docsError, setDocsError] = useState("");
  const apiBase = `${API_ROOT || "http://localhost:8080"}/api/v1`;
  const docSections = [
    { id: "quickstart", title: "快速开始", icon: TerminalSquare },
    { id: "auth", title: "鉴权模型", icon: Key },
    { id: "images", title: "图片上传与分发", icon: Upload },
    { id: "billing", title: "套餐与支付", icon: Box },
    { id: "admin", title: "管理端 API", icon: ShieldCheck },
    { id: "backup", title: "备份与灾备", icon: Archive },
  ];
  const endpointGroups = [
    {
      title: "用户与认证",
      rows: [
        ["POST", "/auth/register", "邮箱注册，开发模式返回验证码。"],
        ["POST", "/auth/login", "邮箱密码登录，返回用户 Bearer Token。"],
        ["POST", "/auth/verify-email", "完成邮箱验证码验证。"],
        ["GET", "/auth/me", "读取当前用户、订阅、用量与鉴权来源。"],
      ],
    },
    {
      title: "对象管理",
      rows: [
        ["POST", "/images", "multipart 上传图片，字段名 file。"],
        ["GET", "/images", "列出当前账号对象。"],
        ["GET", "/images/{public_id}/sign", "签发 15 分钟私有访问 URL。"],
        ["PATCH", "/images/{public_id}/privacy", "切换公开/私有读取。"],
        ["DELETE", "/images/{public_id}", "软删除记录并删除对象文件。"],
      ],
    },
    {
      title: "商业化与开放平台",
      rows: [
        ["POST", "/checkout/ifpay", "创建 IF-Pay 支付订单。"],
        ["GET", "/orders", "读取用户订单状态。"],
        ["GET", "/api-keys", "列出 API Key。"],
        ["POST", "/api-keys", "创建只展示一次的 API Key Secret。"],
        ["DELETE", "/api-keys/{id}", "撤销 API Key。"],
      ],
    },
  ];
  const openReferencePanel = (panel: DocsPanel) => {
    setActivePanel(panel);
    setTimeout(() => document.getElementById("reference-docs")?.scrollIntoView({ behavior: "smooth", block: "start" }), 0);
  };
  useEffect(() => {
    let cancelled = false;
    Promise.all([
      fetch(`${API_ROOT}/docs/openapi.yaml`).then((res) => {
        if (!res.ok) throw new Error(`OpenAPI 文档读取失败 (${res.status})`);
        return res.text();
      }),
      fetch(`${API_ROOT}/docs/api.md`).then((res) => {
        if (!res.ok) throw new Error(`Markdown 文档读取失败 (${res.status})`);
        return res.text();
      }),
    ])
      .then(([openapi, markdown]) => {
        if (cancelled) return;
        setOpenapiText(openapi);
        setMarkdownText(markdown);
        setDocsError("");
      })
      .catch((error) => {
        if (!cancelled) setDocsError(error instanceof Error ? error.message : "技术文档读取失败");
      });
    return () => {
      cancelled = true;
    };
  }, []);
  const renderMarkdownDoc = (source: string) => {
    const lines = source.split("\n");
    return lines.map((line, index) => {
      const key = `${index}-${line.slice(0, 16)}`;
      if (!line.trim()) return <div key={key} className="h-2" />;
      if (line.startsWith("# ")) {
        return <h2 key={key} className="text-2xl font-bold text-[#303133] mt-2 mb-4">{line.replace(/^#\s+/, "")}</h2>;
      }
      if (line.startsWith("## ")) {
        return <h3 key={key} className="text-lg font-bold text-[#303133] mt-6 mb-3 border-l-2 border-[#409EFF] pl-2">{line.replace(/^##\s+/, "")}</h3>;
      }
      if (line.startsWith("- ")) {
        const text = line.replace(/^-\s+/, "");
        const [endpoint, ...rest] = text.split("：");
        return (
          <div key={key} className="flex gap-2 py-1.5 text-sm text-[#606266]">
            <Check size={14} className="text-[#67c23a] shrink-0 mt-0.5" />
            <span>
              <span className="font-mono text-[#409EFF] bg-[#ecf5ff] px-1.5 py-0.5 rounded-sm">{endpoint.replace(/`/g, "")}</span>
              {rest.length > 0 && <span>：{rest.join("：").replace(/`/g, "")}</span>}
            </span>
          </div>
        );
      }
      return <p key={key} className="text-sm text-[#606266] leading-relaxed mb-2">{line.replace(/`/g, "")}</p>;
    });
  };

  return (
    <div className="min-h-screen bg-[#f5f7fa] text-[#303133]">
      <header className="sticky top-0 z-40 bg-white border-b border-[#ebeef5] px-6 py-3 flex items-center justify-between">
        <button type="button" onClick={onBack} className="flex items-center gap-2 text-sm font-semibold text-[#303133] hover:text-[#409EFF]">
          <Database className="text-[#409EFF]" size={22} />
          悦享图床
        </button>
        <div className="flex items-center gap-3">
          <Button variant={activePanel === "openapi" ? "primary" : "secondary"} size="sm" onClick={() => openReferencePanel("openapi")}>OpenAPI YAML</Button>
          <Button variant={activePanel === "markdown" ? "primary" : "secondary"} size="sm" onClick={() => openReferencePanel("markdown")}>Markdown 文档</Button>
          <Button size="sm" onClick={onLogin}>登录控制台</Button>
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-6 py-10 grid lg:grid-cols-[240px_1fr] gap-8">
        <aside className="hidden lg:block">
          <div className="sticky top-24 bg-white border border-[#ebeef5] rounded p-4 shadow-sm">
            <div className="text-xs font-bold text-[#909399] mb-3">技术文档目录</div>
            <nav className="space-y-1">
              {docSections.map((item) => {
                const Icon = item.icon;
                return (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => document.getElementById(item.id)?.scrollIntoView({ behavior: "smooth", block: "start" })}
                    className="w-full flex items-center gap-2 px-3 py-2 rounded text-sm text-[#606266] hover:bg-[#ecf5ff] hover:text-[#409EFF]"
                  >
                    <Icon size={15} />
                    {item.title}
                  </button>
                );
              })}
            </nav>
          </div>
        </aside>

        <div className="space-y-8">
          <section className="bg-white border border-[#ebeef5] rounded p-8 shadow-sm">
            <div className="inline-flex items-center gap-2 px-3 py-1 rounded bg-[#ecf5ff] border border-[#d9ecff] text-[#409EFF] text-xs font-medium mb-5">
              <TerminalSquare size={14} />
              Developer Documentation
            </div>
            <h1 className="text-3xl md:text-4xl font-bold mb-4">技术文档与开放接口</h1>
            <p className="text-sm md:text-base text-[#606266] leading-relaxed max-w-3xl">
              这里是独立的开发者文档页，覆盖用户侧 API、图片上传与分发、IF-Pay 支付回调、API Key、管理端接口和备份灾备。原始 OpenAPI 与 Markdown 文档仍由后端 `/docs/*` 真实提供。
            </p>
            <div className="mt-6 grid sm:grid-cols-3 gap-3 text-xs">
              <div className="border border-[#ebeef5] rounded p-3 bg-[#f8fafc]">
                <div className="text-[#909399] mb-1">API Base</div>
                <div className="font-mono text-[#303133] break-all">{apiBase}</div>
              </div>
              <div className="border border-[#ebeef5] rounded p-3 bg-[#f8fafc]">
                <div className="text-[#909399] mb-1">Image Route</div>
                <div className="font-mono text-[#303133]">/i/{"{public_id}"}</div>
              </div>
              <div className="border border-[#ebeef5] rounded p-3 bg-[#f8fafc]">
                <div className="text-[#909399] mb-1">Auth</div>
                <div className="font-mono text-[#303133]">Bearer / X-API-Key</div>
              </div>
            </div>
          </section>

          <section id="quickstart" className="scroll-mt-24 bg-white border border-[#ebeef5] rounded p-6 shadow-sm">
            <h2 className="text-xl font-bold mb-3">快速开始</h2>
            <p className="text-sm text-[#606266] leading-relaxed mb-4">注册、验证邮箱后即可上传图片。生产环境建议服务端保存用户 Token 或创建 API Key，不要把密钥硬编码到公开客户端。</p>
            <div className="bg-[#282c34] rounded p-4 text-[#abb2bf] font-mono text-xs overflow-auto">
              <pre>{`curl -X POST "${apiBase}/auth/register" \\
  -H "Content-Type: application/json" \\
  -d '{"email":"dev@example.com","password":"Passw0rd!","nickname":"Dev"}'

curl -X POST "${apiBase}/images" \\
  -H "Authorization: Bearer <user_token>" \\
  -F "file=@./banner.png"`}</pre>
            </div>
          </section>

          <section id="auth" className="scroll-mt-24 bg-white border border-[#ebeef5] rounded p-6 shadow-sm">
            <h2 className="text-xl font-bold mb-4">鉴权模型</h2>
            <div className="grid md:grid-cols-3 gap-4">
              {[
                ["用户会话", "Authorization: Bearer <user_token>", "适合控制台操作、订单、备份和个人图片管理。"],
                ["开放 API Key", "X-API-Key: yx_...", "适合服务器上传、列表、删除等自动化任务，可按 scope 控制权限。"],
                ["管理员会话", "Authorization: Bearer yx_admin_...", "首次初始化绑定 TOTP 2FA，后台高危操作都会写入审计日志。"],
              ].map(([title, auth, desc]) => (
                <div key={title} className="border border-[#ebeef5] rounded p-4 bg-[#f8fafc]">
                  <div className="text-sm font-bold mb-2">{title}</div>
                  <div className="font-mono text-xs text-[#409EFF] mb-2 break-all">{auth}</div>
                  <p className="text-xs text-[#606266] leading-relaxed">{desc}</p>
                </div>
              ))}
            </div>
          </section>

          <section id="images" className="scroll-mt-24 bg-white border border-[#ebeef5] rounded p-6 shadow-sm">
            <h2 className="text-xl font-bold mb-4">图片上传与分发</h2>
            <div className="grid lg:grid-cols-2 gap-5">
              <div>
                <p className="text-sm text-[#606266] leading-relaxed mb-4">上传接口返回图片元数据和原图访问链接。公开图可直接通过 <code className="font-mono">/i/{"{public_id}"}</code> 分发；私有图需要先签发短期 URL。</p>
                <div className="space-y-2">
                  {["支持 multipart/form-data 字段 file", "支持公开/私有读取策略切换", "删除时同步清理对象文件和派生图", "分发入口会执行 Referer、签名和冻结状态校验"].map((text) => (
                    <div key={text} className="flex items-center gap-2 text-sm text-[#606266]">
                      <Check size={14} className="text-[#67c23a]" />
                      {text}
                    </div>
                  ))}
                </div>
              </div>
              <div className="bg-[#f8fafc] border border-[#ebeef5] rounded p-4 text-xs font-mono space-y-2">
                <div>GET /i/{"{public_id}"}</div>
                <div>GET /i/{"{public_id}"}/thumbnail.webp</div>
                <div>GET /api/v1/images/{"{public_id}"}/sign</div>
                <div>PATCH /api/v1/images/{"{public_id}"}/privacy</div>
              </div>
            </div>
          </section>

          <section id="billing" className="scroll-mt-24 bg-white border border-[#ebeef5] rounded p-6 shadow-sm">
            <h2 className="text-xl font-bold mb-4">套餐与 IF-Pay 支付</h2>
            <p className="text-sm text-[#606266] leading-relaxed mb-4">套餐通过 `/plans` 公开读取；订购走 IF-Pay OAuth 授权、创建支付单、Webhook 回调、订单入账和订阅发放。后台也提供人工对账、取消、退款接口。</p>
            <div className="grid md:grid-cols-4 gap-3 text-xs">
              {["GET /plans", "GET /oauth/ifpay/start", "POST /checkout/ifpay", "POST /ifpay/webhooks/payments"].map((item) => (
                <div key={item} className="border border-[#ebeef5] rounded p-3 bg-[#f8fafc] font-mono">{item}</div>
              ))}
            </div>
          </section>

          <section id="admin" className="scroll-mt-24 bg-white border border-[#ebeef5] rounded p-6 shadow-sm">
            <h2 className="text-xl font-bold mb-4">管理端 API</h2>
            <p className="text-sm text-[#606266] leading-relaxed mb-4">管理端首次使用必须创建管理员并绑定 2FA。后续用户管理、套餐发放、内容审核、WAF、队列、备份和审计都走管理员会话。</p>
            <div className="grid md:grid-cols-2 gap-3 text-xs font-mono">
              {[
                "POST /admin/auth/bootstrap/start",
                "POST /admin/auth/bootstrap/complete",
                "POST /admin/auth/login",
                "GET /admin/overview",
                "GET /admin/users",
                "POST /admin/images/{public_id}/freeze",
                "PATCH /admin/security/hotlink",
                "GET /admin/audit-logs",
              ].map((item) => <div key={item} className="border border-[#ebeef5] rounded p-3 bg-[#f8fafc]">{item}</div>)}
            </div>
          </section>

          <section id="backup" className="scroll-mt-24 bg-white border border-[#ebeef5] rounded p-6 shadow-sm">
            <h2 className="text-xl font-bold mb-4">备份与灾备</h2>
            <p className="text-sm text-[#606266] leading-relaxed mb-4">用户可导出自己的图片和元数据；管理员可导出全量 ZIP，并在恢复前使用 validate 接口做 manifest/checksum 预检。</p>
            {endpointGroups.map((group) => (
              <div key={group.title} className="mb-5 last:mb-0">
                <h3 className="text-sm font-bold mb-2">{group.title}</h3>
                <div className="overflow-x-auto border border-[#ebeef5] rounded">
                  <table className="w-full text-sm">
                    <tbody>
                      {group.rows.map(([method, path, desc]) => (
                        <tr key={`${method}-${path}`} className="border-b border-[#ebeef5] last:border-0">
                          <td className="px-3 py-2 font-mono text-[#409EFF] whitespace-nowrap">{method}</td>
                          <td className="px-3 py-2 font-mono text-[#303133] whitespace-nowrap">{path}</td>
                          <td className="px-3 py-2 text-xs text-[#606266]">{desc}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            ))}
          </section>

          <section id="reference-docs" className="scroll-mt-24 bg-white border border-[#ebeef5] rounded p-6 shadow-sm">
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-3 mb-5">
              <div>
                <h2 className="text-xl font-bold mb-1">文档原文展示</h2>
                <p className="text-xs text-[#909399]">后端 `/docs/*` 内容在当前页面内渲染，不打开新窗口、不触发下载。</p>
              </div>
              <div className="flex gap-2">
                <Button variant={activePanel === "openapi" ? "primary" : "secondary"} size="sm" onClick={() => setActivePanel("openapi")}>OpenAPI 规范</Button>
                <Button variant={activePanel === "markdown" ? "primary" : "secondary"} size="sm" onClick={() => setActivePanel("markdown")}>接口说明</Button>
              </div>
            </div>

            {docsError && <div className="mb-4 bg-[#fef0f0] border border-[#fde2e2] text-[#f56c6c] px-4 py-3 rounded text-sm">{docsError}</div>}

            {activePanel === "openapi" ? (
              <div className="space-y-5">
                <div className="grid md:grid-cols-3 gap-3">
                  {[
                    ["规范版本", openapiText.match(/openapi:\s*([^\n]+)/)?.[1] || "读取中"],
                    ["接口数量", String((openapiText.match(/^\s{2}\/[^:\n]+:/gm) || []).length || "-")],
                    ["认证方式", "Bearer / X-API-Key / Admin 2FA"],
                  ].map(([label, value]) => (
                    <div key={label} className="bg-[#f8fafc] border border-[#ebeef5] rounded p-4">
                      <div className="text-xs text-[#909399] mb-1">{label}</div>
                      <div className="font-mono text-sm text-[#303133]">{value}</div>
                    </div>
                  ))}
                </div>
                <div className="bg-[#282c34] rounded p-4 text-[#abb2bf] font-mono text-xs overflow-auto max-h-[520px]">
                  <pre>{openapiText || "OpenAPI 规范加载中..."}</pre>
                </div>
              </div>
            ) : (
              <div className="bg-[#fbfdff] border border-[#ebeef5] rounded p-5">
                {markdownText ? renderMarkdownDoc(markdownText) : <div className="text-sm text-[#909399]">接口说明加载中...</div>}
              </div>
            )}
          </section>
        </div>
      </main>
    </div>
  );
};

const AuthView = ({
  onLogin,
  onRegister,
  onVerifyEmail,
  onResendVerification,
  onForgotPassword,
  onResetPassword,
  onStartIFPayOAuth,
  onBack,
}: {
  onLogin: (email: string, password: string) => Promise<void>;
  onRegister: (email: string, password: string, nickname: string) => Promise<RegisterResult>;
  onVerifyEmail: (userID: string, code: string, token: string) => Promise<void>;
  onResendVerification: (userID: string) => Promise<string | undefined>;
  onForgotPassword: (email: string) => Promise<ForgotPasswordResult>;
  onResetPassword: (email: string, code: string, newPassword: string) => Promise<void>;
  onStartIFPayOAuth: () => Promise<void>;
  onBack: () => void;
}) => {
  const [mode, setMode] = useState<"login" | "register" | "verify" | "forgot" | "reset">("login");
  const [pending, setPending] = useState<RegisterResult | null>(null);
  const [resetPending, setResetPending] = useState<ForgotPasswordResult | null>(null);
  const [notice, setNotice] = useState<{ type: ToastType; text: string } | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const inputClass = "w-full px-3 py-2 text-sm rounded border border-[#dcdfe6] focus:border-[#409EFF] focus:outline-none transition-colors";
  const submit = async (task: () => Promise<void>) => {
    setSubmitting(true);
    setNotice(null);
    try {
      await task();
    } catch (error) {
      setNotice({ type: "error", text: error instanceof Error ? error.message : "操作失败，请稍后再试" });
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-6 bg-[#f5f7fa] relative">
      <Button variant="ghost" onClick={onBack} className="absolute top-6 left-6"><ChevronRight className="rotate-180 mr-1" size={16} /> 返回</Button>
      <PanelCard className="w-full max-w-sm p-8 shadow-md">
        <div className="text-center mb-6">
          <Database className="text-[#409EFF] mx-auto mb-3" size={32} strokeWidth={1.5} />
          <h2 className="text-xl font-bold text-[#303133]">{mode === "login" ? "控制台登录" : mode === "register" ? "创建账号" : mode === "verify" ? "邮箱验证" : mode === "forgot" ? "找回密码" : "重置密码"}</h2>
          <p className="text-xs text-[#909399] mt-1">{mode === "verify" ? `验证码已发送至 ${pending?.email || "注册邮箱"}` : mode === "reset" ? `验证码已发送至 ${resetPending?.email || "邮箱"}` : "企业级对象存储与分发控制台"}</p>
        </div>

        {notice && (
          <div className={`mb-4 border px-3 py-2 rounded text-xs ${notice.type === "error" ? "bg-[#fef0f0] border-[#fde2e2] text-[#f56c6c]" : "bg-[#f0f9eb] border-[#e1f3d8] text-[#67c23a]"}`}>
            {notice.text}
          </div>
        )}

        {(mode === "login" || mode === "register") && (
          <div className="grid grid-cols-2 gap-2 mb-5">
            <Button type="button" variant={mode === "login" ? "primary" : "secondary"} size="sm" onClick={() => setMode("login")}>登录</Button>
            <Button type="button" variant={mode === "register" ? "primary" : "secondary"} size="sm" onClick={() => setMode("register")}>注册</Button>
          </div>
        )}

        {mode === "login" && (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              const form = new FormData(e.currentTarget);
              void submit(() => onLogin(String(form.get("email") || ""), String(form.get("password") || "")));
            }}
            className="space-y-4"
          >
            <div>
              <label className="block text-xs font-medium text-[#606266] mb-1">邮箱</label>
              <input name="email" type="email" required className={inputClass} placeholder="name@company.com" />
            </div>
            <div>
              <div className="flex justify-between items-center mb-1">
                <label className="block text-xs font-medium text-[#606266]">访问密码</label>
                <button type="button" className="text-xs text-[#409EFF] hover:underline" onClick={() => setMode("forgot")}>忘记密码?</button>
              </div>
              <input name="password" type="password" required className={inputClass} placeholder="请输入访问密码" />
            </div>
            <Button type="submit" className="w-full mt-2" disabled={submitting}>{submitting ? "登录中..." : "登录"}</Button>
          </form>
        )}

        {mode === "register" && (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              const form = new FormData(e.currentTarget);
              void submit(async () => {
                const result = await onRegister(String(form.get("email") || ""), String(form.get("password") || ""), String(form.get("nickname") || ""));
                setPending(result);
                setMode("verify");
                setNotice({ type: "success", text: result.devCode ? `本地开发验证码：${result.devCode}` : "验证码已发送，请检查邮箱。" });
              });
            }}
            className="space-y-4"
          >
            <div>
              <label className="block text-xs font-medium text-[#606266] mb-1">管理员别名</label>
              <input name="nickname" type="text" required className={inputClass} placeholder="例如：Seron" />
            </div>
            <div>
              <label className="block text-xs font-medium text-[#606266] mb-1">邮箱</label>
              <input name="email" type="email" required className={inputClass} placeholder="name@company.com" />
            </div>
            <div>
              <label className="block text-xs font-medium text-[#606266] mb-1">访问密码</label>
              <input name="password" type="password" required minLength={8} className={inputClass} placeholder="至少 8 位" />
            </div>
            <Button type="submit" className="w-full mt-2" disabled={submitting}>{submitting ? "创建中..." : "创建账号并发送验证码"}</Button>
          </form>
        )}

        {mode === "verify" && (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              const form = new FormData(e.currentTarget);
              if (!pending) {
                setNotice({ type: "error", text: "注册上下文已过期，请重新注册。" });
                return;
              }
              void submit(() => onVerifyEmail(pending.userID, String(form.get("code") || ""), pending.token));
            }}
            className="space-y-4"
          >
            <div>
              <label className="block text-xs font-medium text-[#606266] mb-1">邮箱验证码</label>
              <input name="code" type="text" required className={inputClass} placeholder="输入 6 位验证码" defaultValue={pending?.devCode || ""} />
            </div>
            <Button type="submit" className="w-full mt-2" disabled={submitting}>{submitting ? "验证中..." : "完成验证并进入控制台"}</Button>
            <Button
              type="button"
              variant="secondary"
              className="w-full"
              disabled={submitting || !pending}
              onClick={() => {
                if (!pending) return;
                void submit(async () => {
                  const devCode = await onResendVerification(pending.userID);
                  setPending({ ...pending, devCode });
                  setNotice({ type: "success", text: devCode ? `本地开发验证码：${devCode}` : "验证码已重新发送，请检查邮箱。" });
                });
              }}
            >
              重新发送验证码
            </Button>
            <Button type="button" variant="ghost" className="w-full" onClick={() => setMode("login")}>返回登录</Button>
          </form>
        )}

        {mode === "forgot" && (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              const form = new FormData(e.currentTarget);
              void submit(async () => {
                const result = await onForgotPassword(String(form.get("email") || ""));
                setResetPending(result);
                setMode("reset");
                setNotice({ type: "success", text: result.devCode ? `本地开发验证码：${result.devCode}` : "如果邮箱存在，系统会发送重置验证码。" });
              });
            }}
            className="space-y-4"
          >
            <div>
              <label className="block text-xs font-medium text-[#606266] mb-1">账号邮箱</label>
              <input name="email" type="email" required className={inputClass} placeholder="name@company.com" />
            </div>
            <Button type="submit" className="w-full mt-2" disabled={submitting}>{submitting ? "发送中..." : "发送重置验证码"}</Button>
            <Button type="button" variant="ghost" className="w-full" onClick={() => setMode("login")}>返回登录</Button>
          </form>
        )}

        {mode === "reset" && (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              const form = new FormData(e.currentTarget);
              const email = resetPending?.email || String(form.get("email") || "");
              void submit(async () => {
                await onResetPassword(email, String(form.get("code") || ""), String(form.get("password") || ""));
                setResetPending(null);
                setMode("login");
              });
            }}
            className="space-y-4"
          >
            {!resetPending && (
              <div>
                <label className="block text-xs font-medium text-[#606266] mb-1">账号邮箱</label>
                <input name="email" type="email" required className={inputClass} placeholder="name@company.com" />
              </div>
            )}
            <div>
              <label className="block text-xs font-medium text-[#606266] mb-1">重置验证码</label>
              <input name="code" type="text" required className={inputClass} placeholder="输入验证码" defaultValue={resetPending?.devCode || ""} />
            </div>
            <div>
              <label className="block text-xs font-medium text-[#606266] mb-1">新密码</label>
              <input name="password" type="password" required minLength={8} className={inputClass} placeholder="至少 8 位" />
            </div>
            <Button type="submit" className="w-full mt-2" disabled={submitting}>{submitting ? "重置中..." : "重置密码"}</Button>
            <Button type="button" variant="ghost" className="w-full" onClick={() => setMode("login")}>返回登录</Button>
          </form>
        )}

        <div className="mt-5 flex items-center justify-center gap-3">
          <div className="h-px bg-[#ebeef5] flex-1" />
          <span className="text-xs text-[#909399]">IF-Pay 接入</span>
          <div className="h-px bg-[#ebeef5] flex-1" />
        </div>

        <Button
          type="button"
          variant="secondary"
          className="w-full mt-4"
          disabled={submitting}
          onClick={() => void submit(onStartIFPayOAuth)}
        >
          使用 IF-Pay 授权登录
        </Button>
      </PanelCard>
    </div>
  );
};

type MenuItem = { id: AppView; icon: LucideIcon; label: string } | { divider: true };

const MENU: MenuItem[] = [
  { id: "dashboard", icon: LayoutDashboard, label: "实例监控" },
  { id: "upload", icon: Upload, label: "对象上传" },
  { id: "gallery", icon: FolderOpen, label: "资源管理" },
  { divider: true },
  { id: "pricing", icon: Box, label: "资源包管理" },
  { id: "api", icon: TerminalSquare, label: "API 密钥" },
  { id: "security", icon: ShieldCheck, label: "安全策略" },
  { id: "backup", icon: Archive, label: "数据迁移" },
  { id: "settings", icon: Settings2, label: "实例配置" },
];

const AppLayout = ({
  user,
  usage,
  plans,
  onLogout,
  currentView,
  onNavigate,
  children,
}: {
  user: User;
  usage: APIUsage;
  plans: PricingPlan[];
  onLogout: () => void;
  currentView: AppView;
  onNavigate: (view: AppView) => void;
  children: React.ReactNode;
}) => {
  const currentMenu = MENU.find((item): item is Extract<MenuItem, { id: AppView }> => "id" in item && item.id === currentView);
  const currentPlan = plans.find((plan) => plan.id === user.plan);
  const storagePercent = meterPercent(usage.storage_bytes || 0, currentPlan?.storageBytes);

  return (
    <div className="flex h-screen overflow-hidden bg-[#f5f7fa]">
      <aside className="hidden md:flex w-56 flex-col bg-white border-r border-[#dcdfe6] z-20">
        <div className="h-14 flex items-center gap-2.5 px-5 border-b border-[#ebeef5]">
          <Database className="text-[#409EFF]" size={20} strokeWidth={2} />
          <span className="text-base font-bold text-[#303133]">悦享控制台</span>
        </div>

        <div className="px-4 py-4">
          <div className="p-3 bg-[#f5f7fa] rounded border border-[#ebeef5]">
            <div className="text-xs text-[#606266] font-medium mb-1.5 flex justify-between">
              <span>实例: {user.plan}</span>
              <span className="text-[#409EFF]">{currentPlan?.storageBytes ? `${storagePercent}%` : "∞"}</span>
            </div>
            <div className="w-full bg-[#e4e7ed] rounded-sm h-1 mb-1"><div className="bg-[#409EFF] h-1 rounded-sm" style={{ width: `${currentPlan?.storageBytes ? storagePercent : 100}%` }} /></div>
            <div className="text-[10px] text-[#909399]">{formatBytes(usage.storage_bytes || 0)} / {formatQuotaBytes(currentPlan?.storageBytes)}</div>
          </div>
        </div>

        <nav className="flex-1 overflow-y-auto py-1">
          {MENU.map((item, idx) => {
            if ("divider" in item) {
              return <div key={`div-${idx}`} className="h-px bg-[#ebeef5] my-2 mx-4" />;
            }
            const Icon = item.icon;
            return (
              <button
                key={item.id}
                onClick={() => onNavigate(item.id)}
                className={`w-full flex items-center gap-3 px-5 py-2.5 text-sm transition-colors border-r-2 ${
                  currentView === item.id
                    ? "bg-[#ecf5ff] text-[#409EFF] border-[#409EFF] font-medium"
                    : "text-[#606266] border-transparent hover:bg-[#f5f7fa] hover:text-[#303133]"
                }`}
              >
                <Icon size={16} strokeWidth={2} className={currentView === item.id ? "text-[#409EFF]" : "text-[#909399]"} />
                {item.label}
              </button>
            );
          })}
        </nav>

        <div className="p-4 border-t border-[#ebeef5]">
          <button onClick={onLogout} className="w-full flex items-center gap-2 px-3 py-2 rounded text-sm text-[#606266] hover:bg-[#fef0f0] hover:text-[#f56c6c] transition-colors">
            <LogOut size={16} /> 退出控制台
          </button>
        </div>
      </aside>

      <main className="flex-1 flex flex-col h-full overflow-hidden relative">
        <header className="h-14 bg-white border-b border-[#dcdfe6] flex items-center justify-between px-6 z-10 shrink-0 shadow-sm">
          <h1 className="text-base font-medium text-[#303133]">{currentMenu?.label || "Console"}</h1>
          <div className="flex items-center gap-3">
            <span className="text-xs text-[#909399]">账号: {user.email}</span>
            <div className="w-7 h-7 rounded bg-[#ecf5ff] text-[#409EFF] flex items-center justify-center text-xs font-bold border border-[#b3d8ff]">
              {user.name.charAt(0)}
            </div>
          </div>
        </header>

        <div className="flex-1 overflow-y-auto p-4 md:p-6">
          <div className="max-w-6xl mx-auto h-full">{children}</div>
        </div>
      </main>

      <div className="md:hidden fixed bottom-0 left-0 right-0 bg-white border-t border-[#dcdfe6] flex justify-around p-2 pb-safe z-50">
        {[
          { id: "dashboard" as const, icon: LayoutDashboard, label: "监控" },
          { id: "upload" as const, icon: Upload, label: "上传" },
          { id: "gallery" as const, icon: FolderOpen, label: "资源" },
          { id: "settings" as const, icon: Settings2, label: "配置" },
        ].map((item) => {
          const Icon = item.icon;
          return (
            <button key={item.id} onClick={() => onNavigate(item.id)} className={`flex flex-col items-center gap-1 p-2 ${currentView === item.id ? "text-[#409EFF]" : "text-[#909399]"}`}>
              <Icon size={18} />
              <span className="text-[10px]">{item.label}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
};

const DashboardView = ({ user, usage, plans }: { user: User; usage: APIUsage; plans: PricingPlan[] }) => {
  const currentPlan = plans.find((plan) => plan.id === user.plan);
  const stats: Array<{ label: string; value: string; icon: LucideIcon; color: string; bg: string }> = [
    { label: "本月图片请求", value: formatUsageCount(usage.image_requests || 0), icon: Globe, color: "text-[#e6a23c]", bg: "bg-[#fdf6ec]" },
    { label: "本月下行流量", value: formatQuotaBytes(usage.bandwidth_bytes || 0), icon: Cpu, color: "text-[#409EFF]", bg: "bg-[#ecf5ff]" },
    { label: "节点存储容量", value: formatBytes(usage.storage_bytes || 0), icon: Database, color: "text-[#67c23a]", bg: "bg-[#f0f9eb]" },
    { label: "边缘处理次数", value: formatUsageCount(usage.image_process_events || 0), icon: ShieldAlert, color: "text-[#f56c6c]", bg: "bg-[#fef0f0]" },
  ];

  return (
    <div className="space-y-5">
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {stats.map((stat) => {
          const Icon = stat.icon;
          return (
            <PanelCard key={stat.label} className="flex items-center gap-4 py-4">
              <div className={`p-2.5 rounded ${stat.bg} ${stat.color}`}><Icon size={20} strokeWidth={2} /></div>
              <div>
                <div className="text-xs text-[#909399] mb-0.5">{stat.label}</div>
                <div className="text-xl font-bold text-[#303133]">{stat.value}</div>
              </div>
            </PanelCard>
          );
        })}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
        <PanelCard className="lg:col-span-2">
          <h3 className="text-sm font-bold text-[#303133] mb-5 border-l-4 border-[#409EFF] pl-2">资源消耗水位 (实例: {user.plan})</h3>
          <UsageMeter label="对象存储容量" current={formatBytes(usage.storage_bytes || 0)} max={formatQuotaBytes(currentPlan?.storageBytes)} unit="" percent={meterPercent(usage.storage_bytes || 0, currentPlan?.storageBytes)} colorClass="bg-[#409EFF]" />
          <UsageMeter label="CDN 回源流量" current={formatQuotaBytes(usage.bandwidth_bytes || 0)} max={formatQuotaBytes(currentPlan?.bandwidthBytes)} unit="" percent={meterPercent(usage.bandwidth_bytes || 0, currentPlan?.bandwidthBytes)} colorClass="bg-[#e6a23c]" />
          <UsageMeter label="OpenAPI 调用" current={formatUsageCount(usage.api_calls || 0)} max={formatQuotaCount(currentPlan?.apiCalls)} unit="次" percent={meterPercent(usage.api_calls || 0, currentPlan?.apiCalls)} colorClass="bg-[#67c23a]" />
          <UsageMeter label="边缘计算 (水印/转换)" current={formatUsageCount(usage.image_process_events || 0)} max={formatQuotaCount(currentPlan?.imageProcessEvents)} unit="次" percent={meterPercent(usage.image_process_events || 0, currentPlan?.imageProcessEvents)} colorClass="bg-[#f56c6c]" />
          <div className="mt-5 pt-3 border-t border-[#ebeef5] text-xs text-[#909399]">
            * 统计存在一定延迟，账单周期：自然月。每月 1 日 00:00 (UTC+8) 重置计费项。
          </div>
        </PanelCard>

        <PanelCard>
          <h3 className="text-sm font-bold text-[#303133] mb-4 border-l-4 border-[#409EFF] pl-2">系统状态 / 策略</h3>
          <div className="space-y-3">
            <div className="p-2.5 bg-[#ecf5ff] rounded border border-[#d9ecff]">
              <div className="flex justify-between items-center mb-1">
                <span className="text-[10px] bg-[#409EFF] text-white px-1 rounded">Runtime</span>
                <span className="text-[10px] text-[#909399]">实时</span>
              </div>
              <p className="text-xs text-[#606266]">当前套餐：{currentPlan?.name || user.plan}，上传对象会自动进入私有 Bucket 并触发派生图处理任务。</p>
            </div>
            <div className="p-2.5 bg-[#f5f7fa] rounded border border-[#ebeef5]">
              <div className="flex justify-between items-center mb-1">
                <span className="text-[10px] border border-[#dcdfe6] text-[#909399] px-1 rounded">Policy</span>
                <span className="text-[10px] text-[#909399]">生效中</span>
              </div>
              <p className="text-xs text-[#606266]">公开图仍受防盗链策略保护；私有图需要登录态或短期签名 URL 才能访问。</p>
            </div>
          </div>
        </PanelCard>
      </div>
    </div>
  );
};

const UploadView = ({
  user,
  onUploadFile,
  onUploadSuccess,
  onUploadError,
  onResendVerification,
}: {
  user: User;
  onUploadFile: (file: File) => Promise<ImageItem>;
  onUploadSuccess: (image: ImageItem) => void;
  onUploadError: (message: string) => void;
  onResendVerification: () => Promise<void>;
}) => {
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [resending, setResending] = useState(false);
  const [progress, setProgress] = useState(0);

  const handleUpload = async (file?: File) => {
    if (!file) {
      fileInputRef.current?.click();
      return;
    }
    setUploading(true);
    setProgress(35);
    try {
      const uploaded = await onUploadFile(file);
      setProgress(100);
      setTimeout(() => {
        setUploading(false);
        setProgress(0);
        onUploadSuccess(uploaded);
      }, 250);
    } catch (err) {
      setUploading(false);
      setProgress(0);
      onUploadError(err instanceof Error ? err.message : "上传失败");
    }
  };
  const resendVerification = async () => {
    setResending(true);
    try {
      await onResendVerification();
    } finally {
      setResending(false);
    }
  };

  return (
    <div className="h-full flex flex-col max-w-4xl mx-auto w-full">
      <div className="mb-4">
        <h2 className="text-lg font-bold text-[#303133]">对象上传</h2>
        <p className="text-xs text-[#909399] mt-1">支持拖拽或点击上传，单文件限制：20MB。格式支持：JPG, PNG, GIF, WebP, AVIF, SVG。</p>
      </div>

      {!user.emailVerified && (
        <div className="mb-4 bg-[#fdf6ec] border border-[#faecd8] text-[#e6a23c] px-4 py-3 rounded text-xs flex flex-col sm:flex-row sm:items-center gap-3 justify-between">
          <div className="flex items-start gap-2">
            <AlertTriangle size={16} className="shrink-0 mt-0.5" />
            <span>当前邮箱尚未验证。为保护对象桶写入安全，上传前需要先完成邮箱验证。</span>
          </div>
          <Button variant="secondary" size="sm" disabled={resending} onClick={() => void resendVerification()}>
            {resending ? "发送中..." : "重发验证码"}
          </Button>
        </div>
      )}

      <PanelCard
        className={`flex-1 min-h-[400px] flex flex-col items-center justify-center border-2 border-dashed transition-colors ${isDragging ? "border-[#409EFF] bg-[#ecf5ff]" : "border-[#dcdfe6] hover:border-[#409EFF]"}`}
        onDragOver={(e) => {
          e.preventDefault();
          setIsDragging(true);
        }}
        onDragLeave={() => setIsDragging(false)}
        onDrop={(e) => {
          e.preventDefault();
          setIsDragging(false);
          void handleUpload(e.dataTransfer.files[0]);
        }}
      >
        {uploading ? (
          <div className="w-full max-w-sm text-center">
            <div className="mb-3">
              <Upload className="text-[#409EFF] mx-auto mb-2 animate-bounce" size={28} />
              <h3 className="text-sm text-[#303133]">正在写入至私有 Bucket...</h3>
            </div>
            <div className="w-full bg-[#ebeef5] rounded-sm h-1.5 overflow-hidden">
              <div className="bg-[#409EFF] h-1.5 rounded-sm transition-all duration-150" style={{ width: `${progress}%` }} />
            </div>
            <div className="mt-2 text-xs font-mono text-[#909399]">{progress}% / 100%</div>
          </div>
        ) : (
          <>
            <Upload className="text-[#c0c4cc] mb-4" size={48} strokeWidth={1} />
            <input
              ref={fileInputRef}
              type="file"
              className="hidden"
              accept="image/*,.svg"
              onChange={(e) => {
                void handleUpload(e.target.files?.[0]);
                e.currentTarget.value = "";
              }}
            />
            <div className="text-sm text-[#606266] mb-1">将文件拖到此处，或 <span className="text-[#409EFF] cursor-pointer" onClick={() => void handleUpload()}>点击上传</span></div>
            <p className="text-xs text-[#909399] mb-6">可同时选择多个文件，上传完成后将自动应用 Bucket 默认防盗链策略</p>
            <Button onClick={() => void handleUpload()} size="sm">选择本地文件</Button>
          </>
        )}
      </PanelCard>
    </div>
  );
};

const GalleryView = ({
  images,
  showToast,
  onDeleteImage,
  onTogglePrivacy,
  onSignImage,
}: {
  images: ImageItem[];
  showToast: (msg: string, type?: ToastType) => void;
  onDeleteImage: (publicID: string) => Promise<void>;
  onTogglePrivacy: (publicID: string, nextPrivacy: ImagePrivacy) => Promise<ImageItem>;
  onSignImage: (publicID: string) => Promise<string>;
}) => {
  const [selectedImg, setSelectedImg] = useState<ImageItem | null>(null);
  const [query, setQuery] = useState("");
  const [privacyFilter, setPrivacyFilter] = useState<"all" | ImagePrivacy>("all");

  const filteredImages = images.filter((image) => {
    const keyword = query.trim().toLowerCase();
    const matchesKeyword = !keyword || [image.id, image.name, image.date].some((value) => value.toLowerCase().includes(keyword));
    const matchesPrivacy = privacyFilter === "all" || image.privacy === privacyFilter;
    return matchesKeyword && matchesPrivacy;
  });
  const cyclePrivacyFilter = () => {
    setPrivacyFilter((current) => current === "all" ? "public" : current === "public" ? "private" : "all");
  };

  const copyLink = async (url: string, type: string) => {
    let text = url;
    if (type === "md") text = `![image](${url})`;
    if (type === "html") text = `<img src="${url}" alt="image" />`;
    try {
      if (!navigator.clipboard?.writeText) {
        throw new Error("clipboard_unavailable");
      }
      await navigator.clipboard.writeText(text);
      showToast(`已复制 ${type.toUpperCase()} 格式`, "success");
    } catch {
      showToast("复制失败，请手动复制链接", "error");
    }
  };

  const deleteImage = async (publicID: string) => {
    if (!window.confirm("确认删除该对象？删除后对象文件和派生图会从 Bucket 移除。")) return;
    try {
      await onDeleteImage(publicID);
      setSelectedImg(null);
      showToast("对象已删除", "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "删除失败", "error");
    }
  };

  const togglePrivacy = async (image: ImageItem) => {
    const nextPrivacy: ImagePrivacy = image.privacy === "private" ? "public" : "private";
    try {
      const updated = await onTogglePrivacy(image.id, nextPrivacy);
      setSelectedImg(updated);
      showToast(nextPrivacy === "private" ? "已切换为私有读取" : "已切换为公开读取", "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "权限策略更新失败", "error");
    }
  };

  const signImage = async (publicID: string) => {
    try {
      const signedURL = await onSignImage(publicID);
      void copyLink(signedURL, "url");
      showToast("已复制 15 分钟签名 URL", "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "签名链接生成失败", "error");
    }
  };

  if (selectedImg) {
    return (
      <ImageDetailView
        img={selectedImg}
        onBack={() => setSelectedImg(null)}
        copyLink={copyLink}
        onDelete={() => void deleteImage(selectedImg.id)}
        onTogglePrivacy={() => void togglePrivacy(selectedImg)}
        onSignURL={() => void signImage(selectedImg.id)}
      />
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-col sm:flex-row gap-3 justify-between items-center bg-white p-3 border border-[#dcdfe6] rounded shadow-sm">
        <div className="flex items-center gap-2 w-full sm:w-auto">
          <div className="relative flex-1 sm:w-64">
            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 text-[#c0c4cc]" size={14} />
            <input
              type="text"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="检索对象键 (Object Key)..."
              className="w-full pl-8 pr-3 py-1.5 bg-white rounded-sm border border-[#dcdfe6] focus:border-[#409EFF] focus:outline-none text-xs transition-colors"
            />
          </div>
          <Button variant={privacyFilter === "all" ? "secondary" : "primary"} size="sm" className="px-2" onClick={cyclePrivacyFilter}>
            <Filter size={14} />
            <span className="ml-1 hidden sm:inline">{privacyFilter === "all" ? "全部" : privacyFilter === "public" ? "公开" : "私有"}</span>
          </Button>
        </div>
        <div className="flex gap-2 w-full sm:w-auto justify-end text-xs text-[#909399]">共 {filteredImages.length} / {images.length} 个对象</div>
      </div>

      {images.length === 0 ? (
        <PanelCard className="text-center py-12 border-dashed">
          <FolderOpen className="mx-auto text-[#c0c4cc] mb-3" size={36} strokeWidth={1.5} />
          <div className="text-sm font-medium text-[#303133] mb-1">当前 Bucket 还没有对象</div>
          <div className="text-xs text-[#909399]">上传第一张图片后，这里会展示真实对象、访问链接和审计策略。</div>
        </PanelCard>
      ) : filteredImages.length === 0 ? (
        <PanelCard className="text-center py-12 border-dashed">
          <Search className="mx-auto text-[#c0c4cc] mb-3" size={36} strokeWidth={1.5} />
          <div className="text-sm font-medium text-[#303133] mb-1">没有匹配的对象</div>
          <div className="text-xs text-[#909399]">调整对象键关键词或切换公开/私有筛选后再试。</div>
        </PanelCard>
      ) : (
        <div className="grid grid-cols-2 md:grid-cols-4 xl:grid-cols-6 gap-3">
          {filteredImages.map((img) => (
          <div key={img.id} className="group bg-white border border-[#dcdfe6] rounded-sm overflow-hidden hover:border-[#409EFF] transition-colors cursor-pointer" onClick={() => setSelectedImg(img)}>
            <div className="aspect-square bg-[#f5f7fa] relative overflow-hidden flex items-center justify-center">
              <img src={img.url} alt={img.name} loading="lazy" className="w-full h-full object-cover" />
              <div className="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center gap-2">
                <Search size={20} className="text-white" />
              </div>
              {img.privacy === "private" && <div className="absolute top-1.5 left-1.5 bg-[#303133]/80 p-1 rounded-sm text-white"><Lock size={10} /></div>}
            </div>
            <div className="p-2 border-t border-[#ebeef5]">
              <div className="text-xs font-mono text-[#303133] truncate mb-1" title={img.name}>{img.name}</div>
              <div className="flex justify-between items-center text-[10px] text-[#909399]">
                <span>{img.size}</span>
                <span>{img.date}</span>
              </div>
            </div>
          </div>
          ))}
        </div>
      )}
    </div>
  );
};

const ImageDetailView = ({
  img,
  onBack,
  copyLink,
  onDelete,
  onTogglePrivacy,
  onSignURL,
}: {
  img: ImageItem;
  onBack: () => void;
  copyLink: (url: string, type: string) => Promise<void>;
  onDelete: () => void;
  onTogglePrivacy: () => void;
  onSignURL: () => void;
}) => (
  <div className="space-y-4">
    <div className="flex items-center gap-3 bg-white p-3 border border-[#dcdfe6] rounded shadow-sm">
      <Button variant="ghost" size="sm" onClick={onBack} className="px-1"><ChevronRight className="rotate-180" size={16} /></Button>
      <h2 className="text-sm font-mono font-medium text-[#303133] truncate flex-1">{img.name}</h2>
      {img.privacy === "private" ? (
        <span className="bg-[#fdf6ec] border border-[#faecd8] text-[#e6a23c] text-[10px] px-1.5 py-0.5 rounded-sm flex items-center gap-1"><Lock size={10} /> 私有读取</span>
      ) : (
        <span className="bg-[#f0f9eb] border border-[#e1f3d8] text-[#67c23a] text-[10px] px-1.5 py-0.5 rounded-sm flex items-center gap-1"><Unlock size={10} /> 公开读</span>
      )}
    </div>

    <div className="grid lg:grid-cols-3 gap-4">
      <PanelCard className="lg:col-span-2 flex items-center justify-center bg-[#f5f7fa] min-h-[400px] border-dashed relative">
        <img src={img.url} className="max-w-full max-h-[500px] object-contain shadow-sm border border-[#ebeef5]" alt="Preview" />
        <div className="absolute top-3 right-3 flex gap-2">
          <Button variant="secondary" size="sm" onClick={() => window.open(img.url, "_blank", "noopener,noreferrer")}><Download size={14} /></Button>
          <Button variant="secondary" size="sm" onClick={onTogglePrivacy}>{img.privacy === "private" ? <Unlock size={14} /> : <Lock size={14} />}</Button>
          <Button variant="danger" size="sm" onClick={onDelete}><Trash2 size={14} /></Button>
        </div>
      </PanelCard>

      <div className="space-y-4">
        <PanelCard>
          <h3 className="text-xs font-bold text-[#303133] mb-3 flex items-center gap-1.5 border-l-2 border-[#409EFF] pl-1.5"><LinkIcon size={14} /> 访问链接 (Endpoint)</h3>
          <div className="space-y-2">
            {["url", "md", "html"].map((type) => (
              <div key={type} className="flex gap-1">
                <span className="w-10 shrink-0 text-[10px] text-[#909399] uppercase leading-7 text-right pr-1">{type}</span>
                <input readOnly value={type === "md" ? `![image](${img.url})` : type === "html" ? `<img src="${img.url}"/>` : img.url} className="flex-1 text-[10px] px-2 py-1.5 bg-[#f5f7fa] border border-[#dcdfe6] rounded-sm text-[#606266] font-mono outline-none" />
                <Button variant="secondary" size="sm" className="px-2" onClick={() => void copyLink(img.url, type)}><Copy size={12} /></Button>
              </div>
            ))}
          </div>
          <div className="mt-3 pt-3 border-t border-[#ebeef5]">
            <Button variant="secondary" size="sm" className="w-full text-xs text-[#e6a23c] hover:text-[#e6a23c] border-[#faecd8] bg-[#fdf6ec]" onClick={onSignURL}><Key size={12} className="mr-1.5" /> 颁发短期签名 URL (15M)</Button>
          </div>
        </PanelCard>

        <PanelCard>
          <h3 className="text-xs font-bold text-[#303133] mb-3 flex items-center gap-1.5 border-l-2 border-[#409EFF] pl-1.5"><ShieldCheck size={14} /> 策略与审计</h3>
          <div className="space-y-2 text-xs">
            <div className="flex justify-between items-center py-1.5 border-b border-[#ebeef5]">
              <span className="text-[#606266]">Referer 防盗链</span>
              <span className="text-[#67c23a] flex items-center gap-1"><Check size={12} /> 校验中</span>
            </div>
            <div className="flex justify-between items-center py-1.5 border-b border-[#ebeef5]">
              <span className="text-[#606266]">盲水印注入</span>
              <span className="text-[#409EFF] font-mono">OBJ: {img.id.slice(0, 8)}</span>
            </div>
            <div className="flex justify-between items-center py-1.5">
              <span className="text-[#606266]">感知哈希 (pHash)</span>
              <span className="font-mono text-[#909399]">e2a4...9f1b</span>
            </div>
          </div>
        </PanelCard>
      </div>
    </div>
  </div>
);

const PricingView = ({
  token,
  plans,
  plansError,
  currentPlanSlug,
  ifpayConnected,
  ordersRefreshKey,
  onConnectIFPay,
  onCheckout,
}: {
  token: string;
  plans: PricingPlan[];
  plansError: string;
  currentPlanSlug: string;
  ifpayConnected: boolean;
  ordersRefreshKey: number;
  onConnectIFPay: () => Promise<void>;
  onCheckout: (planSlug: string, billingCycle?: "monthly" | "yearly") => Promise<void>;
}) => {
  const [orders, setOrders] = useState<APIOrder[]>([]);
  const [ordersLoading, setOrdersLoading] = useState(false);
  const [checkingOut, setCheckingOut] = useState("");

  const refreshOrders = async () => {
    if (!token) {
      setOrders([]);
      return;
    }
    setOrdersLoading(true);
    try {
      const rows = await apiFetch<APIOrder[]>("/orders", {}, token);
      setOrders(rows);
    } catch {
      setOrders([]);
    } finally {
      setOrdersLoading(false);
    }
  };

  useEffect(() => {
    let cancelled = false;
    setOrdersLoading(true);
    if (!token) {
      setOrders([]);
      setOrdersLoading(false);
      return;
    }
    apiFetch<APIOrder[]>("/orders", {}, token)
      .then((rows) => {
        if (!cancelled) setOrders(rows);
      })
      .catch(() => {
        if (!cancelled) setOrders([]);
      })
      .finally(() => {
        if (!cancelled) setOrdersLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [token, ordersRefreshKey]);

  const checkout = async (planSlug: string, billingCycle: "monthly" | "yearly") => {
    const key = `${planSlug}:${billingCycle}`;
    setCheckingOut(key);
    try {
      await onCheckout(planSlug, billingCycle);
    } finally {
      setCheckingOut("");
    }
  };

  const orderStatus = (status: string): { label: string; type: TagType } => {
    const map: Record<string, { label: string; type: TagType }> = {
      pending: { label: "待支付", type: "warning" },
      paid: { label: "已支付", type: "success" },
      failed: { label: "支付失败", type: "danger" },
      cancelled: { label: "已取消", type: "info" },
      refunded: { label: "已退款", type: "danger" },
    };
    return map[status] || { label: status, type: "info" };
  };

  return (
  <div className="max-w-4xl mx-auto space-y-5">
    <div className="bg-[#fdf6ec] border border-[#f5dab1] text-[#e6a23c] px-4 py-3 rounded text-xs flex gap-2 items-start">
      <AlertTriangle className="shrink-0 mt-0.5" size={16} />
      <div className="leading-relaxed">
        <strong>生命周期策略通知：</strong> 资源包到期后，关联 Bucket 将自动切入 <strong>30天只读保留状态</strong>（禁止 PUT 请求）。保留期内支持控制台登录及备份导出。逾期 30 天未续费，系统将执行不可逆的 `Destroy` 流程清空数据。
      </div>
    </div>
    {!ifpayConnected && (
      <div className="bg-[#ecf5ff] border border-[#d9ecff] text-[#409EFF] px-4 py-3 rounded text-xs flex flex-col sm:flex-row sm:items-center gap-3 justify-between">
        <span>订购资源包前需要完成 IF-Pay 授权，以便创建支付订单和接收支付回调。</span>
        <Button variant="secondary" size="sm" onClick={() => void onConnectIFPay()}>连接 IF-Pay</Button>
      </div>
    )}

    {plansError && <div className="bg-[#fef0f0] border border-[#fde2e2] text-[#f56c6c] px-4 py-3 rounded text-xs">{plansError}</div>}
    {plans.length === 0 ? (
      <PanelCard className="text-center py-10 border-dashed text-sm text-[#909399]">套餐配置暂不可用，请稍后刷新。</PanelCard>
    ) : (
    <div className="grid md:grid-cols-2 lg:grid-cols-4 gap-4">
      {plans.map((plan) => (
        <PanelCard key={plan.id} className={`flex flex-col relative ${plan.id === "pro" ? "border-[#409EFF] shadow-sm" : ""}`}>
          {plan.id === "pro" && <div className="absolute top-0 right-0 bg-[#409EFF] text-white px-2 py-0.5 rounded-bl text-[10px]">当前执行标准</div>}
          <h3 className="text-sm font-bold text-[#303133] mb-1">{plan.name} 包</h3>
          <div className="flex items-baseline gap-1 mb-2">
            <span className="text-2xl font-bold text-[#409EFF]">¥{plan.priceMo}</span><span className="text-[#909399] text-xs">/ 月</span>
          </div>
          <div className="text-[10px] text-[#909399] mb-4 bg-[#f5f7fa] p-1 rounded-sm text-center">按年结算 ¥{plan.priceYr} (83折)</div>

          <ul className="space-y-2 mb-6 flex-1 text-xs text-[#606266] border-t border-[#ebeef5] pt-3">
            <li className="flex justify-between"><span>对象存储容量</span> <strong className="text-[#303133] font-mono">{plan.storage}</strong></li>
            <li className="flex justify-between"><span>CDN 外网下行</span> <strong className="text-[#303133] font-mono">{plan.traffic}</strong></li>
            <li className="flex justify-between"><span>API & HTTP 请求</span> <strong className="text-[#303133] font-mono">{plan.requests}</strong></li>
            <li className="flex justify-between"><span>边缘处理额度</span> <strong className="text-[#303133] font-mono">{plan.process}</strong></li>
          </ul>

          <div className="grid grid-cols-2 gap-2">
            <Button variant={plan.id === currentPlanSlug ? "secondary" : "primary"} size="sm" className="w-full" disabled={plan.id === currentPlanSlug || checkingOut === `${plan.id}:monthly`} onClick={() => void checkout(plan.id, "monthly")}>
              {plan.id === currentPlanSlug ? "当前套餐" : checkingOut === `${plan.id}:monthly` ? "创建中" : "月付"}
            </Button>
            <Button variant="secondary" size="sm" className="w-full" disabled={plan.id === currentPlanSlug || checkingOut === `${plan.id}:yearly`} onClick={() => void checkout(plan.id, "yearly")}>
              {checkingOut === `${plan.id}:yearly` ? "创建中" : "年付"}
            </Button>
          </div>
        </PanelCard>
      ))}
    </div>
    )}
    <PanelCard>
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-sm font-bold text-[#303133]">最近订单</h3>
        <div className="flex items-center gap-2">
          <Tag type={orders.length ? "primary" : "info"}>{orders.length ? `${orders.length} 条` : "暂无订单"}</Tag>
          <Button variant="secondary" size="sm" className="px-2" disabled={ordersLoading} onClick={() => void refreshOrders()}>
            <RefreshCw size={12} className={ordersLoading ? "animate-spin" : ""} />
          </Button>
        </div>
      </div>
      <div className="space-y-2">
        {orders.slice(0, 3).map((order) => (
          <div key={order.id} className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 border border-[#ebeef5] rounded px-3 py-2 text-xs">
            <div>
              <div className="font-mono text-[#303133]">{order.id}</div>
              <div className="text-[#909399]">{order.plan_slug} · {order.billing_cycle} · {order.ifpay_payment_id || "pending"}</div>
            </div>
            <div className="flex items-center gap-2">
              <Tag type={orderStatus(order.status).type}>{orderStatus(order.status).label}</Tag>
              <span className="text-[#606266]">¥{(order.amount_cent / 100).toFixed(2)}</span>
            </div>
          </div>
        ))}
        {orders.length === 0 && <div className="text-xs text-[#909399]">这里会显示你的 IF-Pay 订单、支付状态和最近一次续费记录。</div>}
      </div>
    </PanelCard>
  </div>
  );
};

const ApiView = ({
  user,
  token,
  showToast,
  onResendVerification,
}: {
  user: User;
  token: string;
  showToast: (msg: string, type?: ToastType) => void;
  onResendVerification: () => Promise<void>;
}) => {
  const [keys, setKeys] = useState<Array<{ id: string; name: string; prefix: string; revoked: boolean; created_at: string; secret?: string }>>([]);
  const [latestSecret, setLatestSecret] = useState("");
  const [resending, setResending] = useState(false);
  const loadKeys = () => {
    if (!token) return;
    apiFetch<Array<{ id: string; name: string; prefix: string; revoked: boolean; created_at: string }>>("/api-keys", {}, token)
      .then(setKeys)
      .catch((err) => showToast(err instanceof Error ? err.message : "API Key 加载失败", "error"));
  };
  useEffect(loadKeys, [token]);
  const createKey = async () => {
    if (!user.emailVerified) {
      showToast("请先完成邮箱验证再创建 API Key", "error");
      return;
    }
    try {
      const data = await apiFetch<{ api_key: { secret: string } }>("/api-keys", {
        method: "POST",
        body: JSON.stringify({ name: "Default Production Key", scopes: ["images:read", "images:write", "albums:read", "albums:write"] }),
      }, token);
      setLatestSecret(data.api_key.secret);
      showToast("API Key 已创建，请立即保存", "success");
      loadKeys();
    } catch (err) {
      showToast(err instanceof Error ? err.message : "API Key 创建失败", "error");
    }
  };
  const revokeKey = async (id: string) => {
    try {
      await apiFetch(`/api-keys/${id}`, { method: "DELETE" }, token);
      showToast("API Key 已撤销", "success");
      loadKeys();
    } catch (err) {
      showToast(err instanceof Error ? err.message : "撤销失败", "error");
    }
  };
  const resendVerification = async () => {
    setResending(true);
    try {
      await onResendVerification();
    } finally {
      setResending(false);
    }
  };
  return (
    <div className="max-w-3xl mx-auto space-y-5">
      {!user.emailVerified && (
        <div className="bg-[#fdf6ec] border border-[#faecd8] text-[#e6a23c] px-4 py-3 rounded text-xs flex flex-col sm:flex-row sm:items-center gap-3 justify-between">
          <div className="flex items-start gap-2">
            <AlertTriangle size={16} className="shrink-0 mt-0.5" />
            <span>当前邮箱尚未验证。为保护长期访问密钥，验证完成后才能创建新的 API Key。</span>
          </div>
          <Button variant="secondary" size="sm" disabled={resending} onClick={() => void resendVerification()}>
            {resending ? "发送中..." : "重发验证码"}
          </Button>
        </div>
      )}
      <PanelCard>
        <div className="flex justify-between items-center mb-4 border-b border-[#ebeef5] pb-3">
          <h3 className="text-sm font-bold text-[#303133] flex items-center gap-1.5 border-l-2 border-[#409EFF] pl-1.5"><Key size={16} /> AccessKey 管理</h3>
          <Button size="sm" variant="secondary" disabled={!user.emailVerified} onClick={() => void createKey()}><RefreshCw size={12} className="mr-1" /> 创建 Key</Button>
        </div>
        {latestSecret && (
          <div className="mb-3 bg-[#282c34] rounded p-3 text-[#98c379] font-mono text-xs break-all">
            {latestSecret}
          </div>
        )}
        <div className="space-y-2">
          {keys.map((key) => (
            <div key={key.id} className="flex items-center justify-between border border-[#ebeef5] rounded p-3 text-xs">
              <div>
                <div className="font-medium text-[#303133]">{key.name}</div>
                <div className="font-mono text-[#909399] mt-1">{key.prefix}****************</div>
              </div>
              <div className="flex items-center gap-2">
                <span className={key.revoked ? "text-[#f56c6c]" : "text-[#67c23a]"}>{key.revoked ? "已撤销" : "启用中"}</span>
                <Button variant="danger" size="sm" disabled={key.revoked} onClick={() => void revokeKey(key.id)}>撤销</Button>
              </div>
            </div>
          ))}
          {keys.length === 0 && <div className="text-xs text-[#909399]">暂无 API Key，点击“创建 Key”生成生产访问密钥。</div>}
        </div>
        <p className="text-[10px] text-[#909399] mt-2">* AccessKey 只在创建后展示一次，服务端仅保存 HMAC 哈希，请勿硬编码在客户端代码或公开 Git 仓库中。</p>
      </PanelCard>

      <PanelCard>
        <h3 className="text-sm font-bold text-[#303133] mb-3 border-l-2 border-[#409EFF] pl-1.5">配置示例 (PicGo JSON)</h3>
        <div className="bg-[#f5f7fa] rounded-sm p-3 font-mono text-xs text-[#606266] border border-[#ebeef5] whitespace-pre-wrap">
{`{
  "picBed": {
    "uploader": "yuexiang-oss",
    "yuexiang-oss": {
      "accessKey": "YOUR_ACCESS_KEY",
      "bucket": "default-prod",
      "region": "cn-east-1",
      "path": "images/{year}/{month}/"
    }
  }
}`}
        </div>
      </PanelCard>
    </div>
  );
};

const SecurityView = () => (
  <div className="max-w-3xl mx-auto space-y-5">
    <div className="bg-[#ecf5ff] border border-[#d9ecff] text-[#409EFF] px-4 py-3 rounded text-xs flex gap-2 items-start">
      <ShieldCheck className="shrink-0 mt-0.5" size={16} />
      <div className="leading-relaxed">
        <strong className="text-[#303133]">业务安全合规声明：</strong> <br />
        系统发现非法 Referer 请求时，将触发 HTTP 403 或返回预设的兜底错误图。若对象被外部恶意保存或分发形成副本，控制台将无法跨域执行 DELETE。针对外溢数据，需依托底层注入的数字盲水印及 pHash 指纹进行链路溯源。
      </div>
    </div>

    <PanelCard>
      <h3 className="text-sm font-bold text-[#303133] mb-4 border-b border-[#ebeef5] pb-3 border-l-2 border-[#409EFF] pl-1.5">边缘安全策略</h3>
      <div className="space-y-5">
        <div className="flex items-center justify-between">
          <div>
            <div className="text-sm font-medium text-[#303133]">Referer 白名单鉴权</div>
            <div className="text-xs text-[#909399] mt-0.5">限制 CDN 节点仅响应来自受信任域的 GET 请求。</div>
          </div>
          <div className="w-10 h-5 bg-[#409EFF] rounded-full relative cursor-pointer"><div className="w-3.5 h-3.5 bg-white rounded-full absolute right-1 top-[3px] shadow-sm" /></div>
        </div>
        <div className="bg-[#f5f7fa] p-2.5 rounded-sm border border-[#ebeef5]">
          <div className="text-[10px] text-[#909399] mb-1.5">ACL 规则表 (支持泛域名匹配)</div>
          <div className="flex gap-1.5 flex-wrap">
            <span className="bg-white border border-[#dcdfe6] px-1.5 py-0.5 rounded-sm text-xs text-[#606266] font-mono">*.yourdomain.com</span>
            <span className="bg-white border border-[#dcdfe6] px-1.5 py-0.5 rounded-sm text-xs text-[#606266] font-mono">localhost:*</span>
          </div>
        </div>

        <div className="flex items-center justify-between pt-3 border-t border-[#ebeef5]">
          <div>
            <div className="text-sm font-medium text-[#303133]">Bucket 强校验模式 (Private Read)</div>
            <div className="text-xs text-[#909399] mt-0.5">拒绝所有匿名访问，请求必须携带有效的 STS Token 或签名参数。</div>
          </div>
          <div className="w-10 h-5 bg-[#dcdfe6] rounded-full relative cursor-pointer"><div className="w-3.5 h-3.5 bg-white rounded-full absolute left-1 top-[3px] shadow-sm" /></div>
        </div>
      </div>
    </PanelCard>

    <PanelCard>
      <h3 className="text-sm font-bold text-[#303133] mb-3 border-l-2 border-[#409EFF] pl-1.5">风险控制服务</h3>
      <div className="grid sm:grid-cols-2 gap-3">
        {[
          { title: "WAF & IP 频控", desc: "基于边缘节点的异常 IP 阻断与 CC 防护" },
          { title: "隐写追踪水印", desc: "图片编解码层面的肉眼不可见身份标识" },
          { title: "pHash 特征库", desc: "违规图库碰撞拦截与变种重传识别" },
          { title: "MFA 设备绑定", desc: "控制台登录启用双重身份验证 (2FA)" },
        ].map((item) => (
          <div key={item.title} className="border border-[#ebeef5] rounded-sm p-3 hover:border-[#409EFF] transition-colors bg-[#f5f7fa]/50">
            <div className="text-xs font-medium text-[#303133] mb-1 flex items-center gap-1.5"><Check size={14} className="text-[#409EFF]" /> {item.title}</div>
            <div className="text-[10px] text-[#909399]">{item.desc}</div>
          </div>
        ))}
      </div>
    </PanelCard>
  </div>
);

const BackupView = ({ token, showToast }: { token: string; showToast: (msg: string, type?: ToastType) => void }) => {
  const importRef = useRef<HTMLInputElement | null>(null);
  const exportBackup = async () => {
    try {
      const res = await fetch(`${API_BASE}/backups/export`, { headers: { Authorization: `Bearer ${token}` } });
      if (!res.ok) throw new Error("备份导出失败");
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `yuexiang-backup-${new Date().toISOString().slice(0, 10)}.zip`;
      a.click();
      URL.revokeObjectURL(url);
      showToast("备份已生成", "success");
    } catch (err) {
      showToast(err instanceof Error ? err.message : "备份导出失败", "error");
    }
  };
  const importBackup = async (file?: File) => {
    if (!file) return;
    const body = new FormData();
    body.append("file", file);
    try {
      const data = await apiFetch<{ restored: number; status: string }>("/backups/import", { method: "POST", body }, token);
      showToast(`导入完成：${data.status}，恢复 ${data.restored} 个对象`, "success");
    } catch (err) {
      showToast(err instanceof Error ? err.message : "备份导入失败", "error");
    }
  };
  return (
    <div className="max-w-3xl mx-auto space-y-5">
      <PanelCard className="text-center py-8">
        <div className="w-12 h-12 bg-[#ecf5ff] border border-[#d9ecff] text-[#409EFF] rounded flex items-center justify-center mx-auto mb-3">
          <Archive size={24} />
        </div>
        <h3 className="text-sm font-bold text-[#303133] mb-2">生成数据快照 (Export ZIP)</h3>
        <p className="text-[#909399] text-xs mb-5 max-w-md mx-auto">
          将当前账号图片对象、元数据、Manifest 与 checksums.sha256 打包下载，便于异地灾备或迁移。
        </p>
        <Button onClick={() => void exportBackup()} size="sm">下载备份 ZIP</Button>
      </PanelCard>

      <PanelCard>
        <h3 className="text-sm font-bold text-[#303133] mb-3 border-b border-[#ebeef5] pb-2 border-l-2 border-[#409EFF] pl-1.5">归档数据导入 (Import)</h3>
        <input
          ref={importRef}
          type="file"
          className="hidden"
          accept=".zip,application/zip"
          onChange={(e) => {
            void importBackup(e.target.files?.[0]);
            e.currentTarget.value = "";
          }}
        />
        <div className="border border-dashed border-[#c0c4cc] rounded-sm p-6 text-center hover:border-[#409EFF] hover:bg-[#f5f7fa] transition-colors cursor-pointer bg-[#fafafa]" onClick={() => importRef.current?.click()}>
          <FileJson className="mx-auto text-[#c0c4cc] mb-2" size={28} strokeWidth={1.5} />
          <div className="text-sm font-medium text-[#606266]">点击上传 Backup Archive (ZIP) 并解析恢复</div>
          <div className="text-[10px] text-[#909399] mt-1">校验 manifest.json 与 checksums.sha256 后恢复对象</div>
        </div>
        <div className="mt-3 p-2 bg-[#fdf6ec] border border-[#f5dab1] rounded-sm text-[10px] text-[#e6a23c]">
          * 导入会为当前账号生成新的 Public ID，不会覆盖线上已有对象。
        </div>
      </PanelCard>
    </div>
  );
};

const SettingsView = ({
  user,
  token,
  showToast,
  onUserUpdate,
  onResendVerification,
}: {
  user: User;
  token: string;
  showToast: (msg: string, type?: ToastType) => void;
  onUserUpdate: (user: User) => void;
  onResendVerification: () => Promise<void>;
}) => {
  const [alias, setAlias] = useState(user.name);
  const [avatarURL, setAvatarURL] = useState(user.avatarURL || "");
  const [destroyReason, setDestroyReason] = useState("");
  const [resending, setResending] = useState(false);
  const avatarInputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    setAlias(user.name);
    setAvatarURL(user.avatarURL || "");
  }, [user.avatarURL, user.name]);

  const handleAvatarFile = (file?: File) => {
    if (!file) return;
    if (!file.type.startsWith("image/")) {
      showToast("请选择图片文件作为 Avatar", "error");
      return;
    }
    if (file.size > 512 * 1024) {
      showToast("Avatar 图片请控制在 512KB 以内", "error");
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      const nextURL = String(reader.result || "");
      if (!nextURL.startsWith("data:image/") || nextURL.length > 512 * 1024) {
        showToast("图片编码后超过限制，请压缩后重试", "error");
        return;
      }
      setAvatarURL(nextURL);
      showToast("Avatar 已预览，保存配置后生效", "success");
    };
    reader.onerror = () => showToast("Avatar 读取失败，请重试", "error");
    reader.readAsDataURL(file);
  };

  const saveProfile = async () => {
    try {
      const data = await apiFetch<{ user: APIUser }>("/settings/profile", {
        method: "PATCH",
        body: JSON.stringify({ nickname: alias.trim(), avatar_url: avatarURL }),
      }, token);
      onUserUpdate(mapUser(data.user));
      showToast("配置已保存", "success");
    } catch (err) {
      showToast(err instanceof Error ? err.message : "保存失败", "error");
    }
  };

  const requestDestroy = async () => {
    if (!window.confirm("WARN: 确认提交账号销毁工单？提交后仍需人工复核，线上数据不会立即删除。")) return;
    try {
      const data = await apiFetch<{ ticket_id: string; status: string }>("/settings/account-destroy-request", {
        method: "POST",
        body: JSON.stringify({ reason: destroyReason.trim() }),
      }, token);
      showToast(`销毁工单已提交：${data.ticket_id}`, "error");
    } catch (err) {
      showToast(err instanceof Error ? err.message : "工单提交失败", "error");
    }
  };
  const resendVerification = async () => {
    setResending(true);
    try {
      await onResendVerification();
    } finally {
      setResending(false);
    }
  };

  return (
    <div className="max-w-3xl mx-auto space-y-5">
      <PanelCard>
        <h3 className="text-sm font-bold text-[#303133] mb-4 border-l-2 border-[#409EFF] pl-1.5">基本配置</h3>
        <div className="space-y-4">
          <div className="flex items-center gap-5">
            <div className="w-14 h-14 bg-[#409EFF] rounded text-lg text-white font-bold flex items-center justify-center overflow-hidden border border-[#d9ecff]">
              {avatarURL ? (
                <img src={avatarURL} alt="Avatar" className="w-full h-full object-cover" />
              ) : (
                user.name.charAt(0)
              )}
            </div>
            <input
              ref={avatarInputRef}
              type="file"
              accept="image/*"
              className="hidden"
              onChange={(event) => {
                handleAvatarFile(event.target.files?.[0]);
                event.currentTarget.value = "";
              }}
            />
            <div className="flex gap-2">
              <Button variant="secondary" size="sm" onClick={() => avatarInputRef.current?.click()}>上传新 Avatar</Button>
              {avatarURL && <Button variant="ghost" size="sm" onClick={() => setAvatarURL("")}>移除</Button>}
            </div>
          </div>
          <div className="grid gap-3 pt-2">
            <div>
              <label className="block text-xs font-medium text-[#606266] mb-1">管理员别名 (Alias)</label>
              <input type="text" value={alias} onChange={(event) => setAlias(event.target.value)} className="w-full max-w-sm px-2.5 py-1.5 text-sm border border-[#dcdfe6] rounded-sm focus:border-[#409EFF] focus:outline-none" />
            </div>
            <div>
              <label className="block text-xs font-medium text-[#606266] mb-1">主告警邮箱 (Primary Email)</label>
              <input type="email" disabled value={user.email} className="w-full max-w-sm px-2.5 py-1.5 text-sm border border-[#ebeef5] bg-[#f5f7fa] text-[#909399] rounded-sm cursor-not-allowed" />
            </div>
            <div className="max-w-sm p-3 border border-[#ebeef5] rounded-sm bg-[#f5f7fa] flex items-center justify-between gap-3">
              <div>
                <div className="text-xs font-medium text-[#606266]">邮箱验证状态</div>
                <div className={`text-xs mt-1 ${user.emailVerified ? "text-[#67c23a]" : "text-[#e6a23c]"}`}>
                  {user.emailVerified ? "已验证，可上传对象和创建 API Key" : "未验证，上传写入会被限制"}
                </div>
              </div>
              {!user.emailVerified && (
                <Button variant="secondary" size="sm" disabled={resending} onClick={() => void resendVerification()}>
                  {resending ? "发送中" : "重发验证码"}
                </Button>
              )}
            </div>
          </div>
          <Button size="sm" className="mt-1" onClick={() => void saveProfile()}>保存配置</Button>
        </div>
      </PanelCard>

      <PanelCard className="border-[#fde2e2] bg-[#fef0f0]">
        <h3 className="text-sm font-bold text-[#f56c6c] mb-2">高危操作区 (Danger Zone)</h3>
        <p className="text-xs text-[#606266] mb-3">销毁账号会先创建人工复核工单并写入审计日志。真正硬删除应在备份确认、账务结清和维护窗口内执行。</p>
        <textarea value={destroyReason} onChange={(event) => setDestroyReason(event.target.value)} className="w-full max-w-lg h-20 px-2.5 py-1.5 text-xs border border-[#fde2e2] rounded-sm focus:border-[#f56c6c] focus:outline-none mb-3" placeholder="可选：填写注销原因，便于人工复核" />
        <div><Button variant="danger" size="sm" onClick={() => void requestDestroy()}>申请销毁账号</Button></div>
      </PanelCard>
    </div>
  );
};

export default App;
