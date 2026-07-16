import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Activity,
  Cloud,
  Database,
  Gauge,
  Play,
  Plus,
  RefreshCw,
  Server,
  Settings2,
  Square,
  Trash2,
  Zap,
  CheckCircle2,
  XCircle,
  Loader2,
  HardDrive,
} from 'lucide-react'

const tabs = [
  { id: 'progress', label: 'Active Progress', icon: Activity },
  { id: 'rules', label: 'Rules', icon: Settings2 },
  { id: 'providers', label: 'Providers', icon: Server },
]

// Keep API token in process memory only (never sessionStorage/localStorage).
// CodeQL: js/clear-text-storage-of-sensitive-data — avoid browser durable storage.
let memoryAPIToken = ''

function getStoredToken() {
  return memoryAPIToken
}

function setStoredToken(t) {
  memoryAPIToken = t ? String(t) : ''
}

function authHeaders() {
  const t = getStoredToken()
  if (!t) return {}
  return { Authorization: `Bearer ${t}` }
}

async function api(path, opts = {}) {
  const res = await fetch(path, {
    headers: {
      'Content-Type': 'application/json',
      ...authHeaders(),
      ...(opts.headers || {}),
    },
    ...opts,
  })
  const text = await res.text()
  let data = null
  try {
    data = text ? JSON.parse(text) : null
  } catch {
    data = { error: text }
  }
  if (res.status === 401) {
    const err = new Error(data?.error || 'unauthorized')
    err.status = 401
    throw err
  }
  if (!res.ok) {
    throw new Error(data?.error || res.statusText || 'request failed')
  }
  return data
}

function formatBytes(n) {
  if (!n || n <= 0) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = n
  while (v >= 1024 && i < u.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${u[i]}`
}

function formatSpeed(bps) {
  if (!bps || bps <= 0) return '0 B/s'
  return `${formatBytes(bps)}/s`
}

function etaSeconds(remainingBytes, bps) {
  if (!bps || bps <= 0 || !remainingBytes) return null
  return remainingBytes / bps
}

function formatEta(sec) {
  if (sec == null || !Number.isFinite(sec)) return '—'
  if (sec < 60) return `${Math.ceil(sec)}s`
  if (sec < 3600) return `${Math.floor(sec / 60)}m ${Math.ceil(sec % 60)}s`
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  return `${h}h ${m}m`
}

const emptyProvider = {
  name: '',
  provider_type: 's3',
  endpoint: '',
  region: 'us-east-1',
  access_key_id: '',
  secret_access_key: '',
}

// Known S3-compatible providers with sensible defaults (all fields remain editable).
const KNOWN_PROVIDERS = [
  {
    id: 'manual',
    label: 'Custom / manual…',
    provider_type: 's3',
    endpoint: '',
    region: 'us-east-1',
    name: '',
    hint: 'Enter any S3-compatible endpoint, region, and credentials.',
  },
  {
    id: 'aws',
    label: 'Amazon Web Services (S3)',
    provider_type: 'aws',
    endpoint: '',
    region: 'us-east-1',
    name: 'AWS S3',
    hint: 'Leave endpoint empty. Set region to match your buckets (e.g. eu-west-1).',
  },
  {
    id: 'r2',
    label: 'Cloudflare R2',
    provider_type: 'r2',
    endpoint: 'https://<ACCOUNT_ID>.r2.cloudflarestorage.com',
    region: 'auto',
    name: 'Cloudflare R2',
    hint: 'Replace <ACCOUNT_ID> with your Cloudflare account ID from the R2 dashboard.',
  },
  {
    id: 'arvancloud',
    label: 'ArvanCloud Object Storage',
    provider_type: 'arvancloud',
    endpoint: 'https://s3.ir-thr-at1.arvanstorage.ir',
    region: 'ir-thr-at1',
    name: 'ArvanCloud',
    hint: 'Default Tehran (Simin). Other regions include ir-tbz-sh1 (Tabriz).',
  },
  {
    id: 'minio',
    label: 'MinIO',
    provider_type: 'minio',
    endpoint: 'http://127.0.0.1:9000',
    region: 'us-east-1',
    name: 'MinIO',
    hint: 'Point endpoint at your MinIO host. Path-style addressing is used automatically.',
  },
  {
    id: 'alibaba',
    label: 'Alibaba Cloud OSS',
    provider_type: 'alibaba',
    endpoint: 'https://oss-cn-hangzhou.aliyuncs.com',
    region: 'oss-cn-hangzhou',
    name: 'Alibaba OSS',
    hint: 'Pick the OSS regional endpoint for your bucket (Hangzhou default).',
  },
  {
    id: 'parspack',
    label: 'Parspack',
    provider_type: 'parspack',
    endpoint: '',
    region: 'default',
    name: 'Parspack',
    hint: 'Copy Access Key, Secret, and S3 endpoint from the Parspack cloud storage panel.',
  },
  {
    id: 'hetzner',
    label: 'Hetzner Object Storage',
    provider_type: 'hetzner',
    endpoint: 'https://fsn1.your-objectstorage.com',
    region: 'fsn1',
    name: 'Hetzner',
    hint: 'Match location: fsn1 (Falkenstein), nbg1 (Nuremberg), or hel1 (Helsinki).',
  },
  {
    id: 'dunkel',
    label: 'Dunkel Cloud Storage',
    provider_type: 'dunkel',
    endpoint: 'https://s3.dunkel.de',
    region: 'de',
    name: 'Dunkel',
    hint: 'German S3-compatible storage. Confirm endpoint in the Dunkel console if different.',
  },
  {
    id: 'b2',
    label: 'Backblaze B2',
    provider_type: 'b2',
    endpoint: 'https://s3.us-west-004.backblazeb2.com',
    region: 'us-west-004',
    name: 'Backblaze B2',
    hint: 'Use the S3-compatible endpoint shown for your B2 region in the console.',
  },
  {
    id: 'wasabi',
    label: 'Wasabi',
    provider_type: 'wasabi',
    endpoint: 'https://s3.wasabisys.com',
    region: 'us-east-1',
    name: 'Wasabi',
    hint: 'Regional endpoints e.g. https://s3.eu-central-1.wasabisys.com with matching region.',
  },
  {
    id: 'digitalocean',
    label: 'DigitalOcean Spaces',
    provider_type: 'digitalocean',
    endpoint: 'https://nyc3.digitaloceanspaces.com',
    region: 'nyc3',
    name: 'DigitalOcean Spaces',
    hint: 'Change the region subdomain (nyc3, ams3, sgp1, sfo3, …).',
  },
  {
    id: 'linode',
    label: 'Akamai / Linode Object Storage',
    provider_type: 'linode',
    endpoint: 'https://us-east-1.linodeobjects.com',
    region: 'us-east-1',
    name: 'Linode Objects',
    hint: 'Use the cluster endpoint from Cloud Manager (us-east-1, eu-central-1, …).',
  },
  {
    id: 'scaleway',
    label: 'Scaleway Object Storage',
    provider_type: 'scaleway',
    endpoint: 'https://s3.nl-ams.scw.cloud',
    region: 'nl-ams',
    name: 'Scaleway',
    hint: 'Regions: nl-ams, fr-par, pl-waw, it-mil.',
  },
  {
    id: 'ovh',
    label: 'OVHcloud Object Storage',
    provider_type: 'ovh',
    endpoint: 'https://s3.gra.io.cloud.ovh.net',
    region: 'gra',
    name: 'OVHcloud',
    hint: 'Region codes include gra, rbx, sbg, de, waw, bhs, …',
  },
  {
    id: 'gcs',
    label: 'Google Cloud Storage (HMAC / S3 API)',
    provider_type: 'gcs',
    endpoint: 'https://storage.googleapis.com',
    region: 'auto',
    name: 'GCS',
    hint: 'Create HMAC keys for a service account in Cloud Storage settings.',
  },
  {
    id: 'liara',
    label: 'Liara Object Storage',
    provider_type: 'liara',
    endpoint: 'https://storage.iran.liara.space',
    region: 'us-east-1',
    name: 'Liara',
    hint: 'Confirm the exact endpoint in the Liara object storage panel.',
  },
  {
    id: 'ionos',
    label: 'IONOS Object Storage',
    provider_type: 'ionos',
    endpoint: 'https://s3.eu-central-1.ionoscloud.com',
    region: 'eu-central-1',
    name: 'IONOS',
    hint: 'Use the IONOS regional S3 endpoint for your contract location.',
  },
  {
    id: 'idrive',
    label: 'IDrive e2',
    provider_type: 'idrive',
    endpoint: 'https://l1n3.va.idrivee2-12.com',
    region: 'us-east-1',
    name: 'IDrive e2',
    hint: 'Copy the endpoint hostname from the IDrive e2 storage region page.',
  },
]

function findKnownProvider(id) {
  return KNOWN_PROVIDERS.find((p) => p.id === id) || KNOWN_PROVIDERS[0]
}

const emptyRule = {
  name: '',
  source_provider_id: '',
  source_bucket: '',
  source_prefix: '',
  target_provider_id: '',
  target_bucket: '',
  target_prefix: '',
  extra_targets: '',
  include_patterns: '',
  exclude_patterns: '',
  min_size_bytes: '',
  max_size_bytes: '',
  modified_after: '',
  delete_on_target: false,
  concurrency_limit: 4,
  bandwidth_limit_kbps: 0,
  schedule_cron: '',
  schedule_enabled: false,
  compare_mode: 'etag',
}

export default function App() {
  const [tab, setTab] = useState('progress')
  const [providers, setProviders] = useState([])
  const [rules, setRules] = useState([])
  const [jobs, setJobs] = useState([])
  const [logs, setLogs] = useState([])
  const [progress, setProgress] = useState(null)
  const [workers, setWorkers] = useState([])
  const [version, setVersion] = useState(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [needsAuth, setNeedsAuth] = useState(false)
  const [tokenInput, setTokenInput] = useState(getStoredToken())
  const [authReady, setAuthReady] = useState(false)

  const [providerForm, setProviderForm] = useState(emptyProvider)
  const [providerPreset, setProviderPreset] = useState('manual')
  const [testResult, setTestResult] = useState(null)
  const [ruleForm, setRuleForm] = useState(emptyRule)
  const [dryRun, setDryRun] = useState(null)
  const [selectedRuleId, setSelectedRuleId] = useState('')

  const logEndRef = useRef(null)

  const refresh = useCallback(async () => {
    try {
      const [p, r, j, v] = await Promise.all([
        api('/api/providers'),
        api('/api/rules'),
        api('/api/jobs'),
        api('/api/version'),
      ])
      setProviders(p || [])
      setRules(r || [])
      setJobs(j || [])
      setVersion(v)
      setError('')
      setNeedsAuth(false)
      setAuthReady(true)
    } catch (e) {
      if (e.status === 401) {
        setNeedsAuth(true)
        setAuthReady(true)
        setError('Authentication required. Enter your API token.')
        return
      }
      setError(e.message)
      setAuthReady(true)
    }
  }, [])

  useEffect(() => {
    refresh()
  }, [refresh])

  useEffect(() => {
    if (needsAuth) return undefined
    const token = getStoredToken()
    const url = token
      ? `/api/jobs/stream?access_token=${encodeURIComponent(token)}`
      : '/api/jobs/stream'
    const es = new EventSource(url)
    es.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data)
        if (msg.type === 'log' || msg.type === 'error' || msg.type === 'job') {
          setLogs((prev) => {
            const next = [...prev, { t: msg.timestamp || new Date().toISOString(), m: msg.message || msg.type, job: msg.job_id }]
            return next.slice(-400)
          })
          if (msg.type === 'job') refresh()
        }
        if (msg.type === 'progress' && msg.payload) {
          setProgress(msg.payload)
          setWorkers(msg.payload.workers || [])
        }
      } catch {
        /* ignore */
      }
    }
    es.onerror = () => {
      /* browser will retry */
    }
    return () => es.close()
  }, [refresh, needsAuth, tokenInput])

  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [logs])

  const pct = useMemo(() => {
    if (!progress) return 0
    const total = progress.total_bytes || 0
    const done = progress.bytes_transferred || 0
    if (total <= 0) {
      const tf = progress.total_files || 0
      const fd = (progress.files_done || 0) + (progress.files_failed || 0)
      return tf > 0 ? Math.min(100, (fd / tf) * 100) : 0
    }
    return Math.min(100, (done / total) * 100)
  }, [progress])

  const remaining = (progress?.total_bytes || 0) - (progress?.bytes_transferred || 0)
  const eta = etaSeconds(remaining > 0 ? remaining : 0, progress?.bytes_per_sec)

  function applyProviderPreset(presetId) {
    setProviderPreset(presetId)
    const preset = findKnownProvider(presetId)
    if (presetId === 'manual') {
      setProviderForm((f) => ({
        ...emptyProvider,
        access_key_id: f.access_key_id,
        secret_access_key: f.secret_access_key,
      }))
      return
    }
    setProviderForm((f) => ({
      ...f,
      name: preset.name || f.name,
      provider_type: preset.provider_type,
      endpoint: preset.endpoint,
      region: preset.region,
    }))
    setTestResult(null)
  }

  async function saveProvider(e) {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      await api('/api/providers', { method: 'POST', body: JSON.stringify(providerForm) })
      setProviderForm(emptyProvider)
      setProviderPreset('manual')
      setTestResult(null)
      await refresh()
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  async function testProvider(e) {
    e.preventDefault()
    setBusy(true)
    setTestResult(null)
    try {
      const res = await api('/api/providers/test', { method: 'POST', body: JSON.stringify(providerForm) })
      setTestResult(res)
    } catch (err) {
      setTestResult({ ok: false, error: err.message })
    } finally {
      setBusy(false)
    }
  }

  async function deleteProvider(id) {
    if (!confirm('Delete this provider?')) return
    try {
      await api(`/api/providers/${id}`, { method: 'DELETE' })
      await refresh()
    } catch (err) {
      setError(err.message)
    }
  }

  async function saveRule(e) {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      const body = {
        ...ruleForm,
        concurrency_limit: Number(ruleForm.concurrency_limit) || 4,
        bandwidth_limit_kbps: Number(ruleForm.bandwidth_limit_kbps) || 0,
        min_size_bytes: ruleForm.min_size_bytes === '' ? null : Number(ruleForm.min_size_bytes),
        max_size_bytes: ruleForm.max_size_bytes === '' ? null : Number(ruleForm.max_size_bytes),
        modified_after: ruleForm.modified_after || null,
      }
      const saved = await api('/api/rules', { method: 'POST', body: JSON.stringify(body) })
      setSelectedRuleId(saved.id)
      setDryRun(null)
      await refresh()
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  async function runDryRun(ruleId) {
    setBusy(true)
    setError('')
    try {
      const res = await api(`/api/rules/${ruleId}/dry-run`, { method: 'POST', body: '{}' })
      setDryRun(res)
      setSelectedRuleId(ruleId)
      setTab('rules')
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  async function startJob(ruleId) {
    setBusy(true)
    setError('')
    try {
      await api(`/api/rules/${ruleId}/start`, { method: 'POST', body: '{}' })
      setTab('progress')
      setLogs((prev) => [...prev, { t: new Date().toISOString(), m: 'job start requested', job: ruleId }])
      await refresh()
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  async function cancelJob(jobId) {
    setBusy(true)
    setError('')
    try {
      await api(`/api/jobs/${jobId}/cancel`, { method: 'POST', body: '{}' })
      setLogs((prev) => [...prev, { t: new Date().toISOString(), m: `cancel requested for ${jobId.slice(0, 8)}`, job: jobId }])
      await refresh()
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  function submitToken(e) {
    e.preventDefault()
    setStoredToken(tokenInput.trim())
    setNeedsAuth(false)
    setError('')
    refresh()
  }

  function clearToken() {
    setStoredToken('')
    setTokenInput('')
    refresh()
  }

  if (!authReady) {
    return (
      <div className="min-h-screen flex items-center justify-center text-slate-400 text-sm">
        <Loader2 className="h-5 w-5 animate-spin mr-2" /> Loading…
      </div>
    )
  }

  if (needsAuth) {
    return (
      <div className="min-h-screen flex items-center justify-center px-4">
        <form onSubmit={submitToken} className="w-full max-w-md rounded-xl border border-slate-800 bg-surface-800/80 p-6 space-y-4 shadow-xl">
          <div className="flex items-center gap-3">
            <div className="h-10 w-10 rounded-lg bg-gradient-to-br from-blue-500 to-cyan-400 flex items-center justify-center">
              <Zap className="h-5 w-5 text-white" />
            </div>
            <div>
              <h1 className="text-lg font-semibold">Gantry</h1>
              <p className="text-xs text-slate-400">API token required</p>
            </div>
          </div>
          <p className="text-sm text-slate-400">
            This instance has authentication enabled. Enter the shared API token configured with <code className="text-slate-300">-api-token</code> / <code className="text-slate-300">GANTRY_API_TOKEN</code>.
          </p>
          {error && <p className="text-sm text-red-300">{error}</p>}
          <label className="block text-xs text-slate-400">
            API token
            <input
              type="password"
              autoFocus
              className="mt-1 w-full rounded-md bg-surface-900 border border-slate-700 px-3 py-2 text-sm"
              value={tokenInput}
              onChange={(e) => setTokenInput(e.target.value)}
              required
            />
          </label>
          <button type="submit" className="w-full rounded-md bg-blue-600 hover:bg-blue-500 py-2 text-sm font-medium">
            Continue
          </button>
        </form>
      </div>
    )
  }

  return (
    <div className="min-h-screen flex flex-col">
      <header className="border-b border-slate-800/80 bg-surface-800/70 backdrop-blur sticky top-0 z-20">
        <div className="max-w-7xl mx-auto px-4 py-3 flex items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="h-9 w-9 rounded-lg bg-gradient-to-br from-blue-500 to-cyan-400 flex items-center justify-center shadow-lg shadow-blue-500/20">
              <Zap className="h-5 w-5 text-white" />
            </div>
            <div>
              <h1 className="text-lg font-semibold tracking-tight">Gantry</h1>
              <p className="text-xs text-slate-400">Multi-provider S3 sync engine</p>
            </div>
          </div>
          <div className="flex items-center gap-3 text-xs text-slate-400">
            {version && (
              <span className="hidden sm:inline font-mono px-2 py-1 rounded bg-surface-700/80 border border-slate-700">
                v{version.version}
              </span>
            )}
            {getStoredToken() && (
              <button type="button" onClick={clearToken} className="px-2 py-1 rounded border border-slate-700 hover:bg-surface-700">
                Clear token
              </button>
            )}
            <button
              type="button"
              onClick={refresh}
              className="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-md border border-slate-700 hover:bg-surface-700 transition"
            >
              <RefreshCw className="h-3.5 w-3.5" /> Refresh
            </button>
          </div>
        </div>
        <nav className="max-w-7xl mx-auto px-4 flex gap-1 pb-2">
          {tabs.map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              type="button"
              onClick={() => setTab(id)}
              className={`inline-flex items-center gap-2 px-3 py-2 rounded-md text-sm transition ${
                tab === id
                  ? 'bg-blue-600/20 text-blue-300 border border-blue-500/40'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-surface-700/60 border border-transparent'
              }`}
            >
              <Icon className="h-4 w-4" /> {label}
            </button>
          ))}
        </nav>
      </header>

      <main className="flex-1 max-w-7xl w-full mx-auto px-4 py-6 space-y-4">
        {error && (
          <div className="rounded-lg border border-red-500/40 bg-red-950/40 px-4 py-3 text-sm text-red-200 flex items-start gap-2">
            <XCircle className="h-4 w-4 mt-0.5 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {tab === 'progress' && (
          <section className="space-y-4">
            <div className="grid md:grid-cols-3 gap-4">
              <StatCard icon={Gauge} label="Throughput" value={formatSpeed(progress?.bytes_per_sec)} />
              <StatCard icon={HardDrive} label="Transferred" value={formatBytes(progress?.bytes_transferred)} sub={`/ ${formatBytes(progress?.total_bytes)}`} />
              <StatCard icon={Activity} label="ETA" value={formatEta(eta)} sub={`${progress?.files_done || 0} files done`} />
            </div>

            <div className="rounded-xl border border-slate-800 bg-surface-800/60 p-5">
              <div className="flex justify-between text-sm mb-2">
                <span className="text-slate-300 font-medium">Global progress</span>
                <span className="font-mono text-slate-400">{pct.toFixed(1)}%</span>
              </div>
              <div className="h-3 rounded-full bg-surface-900 overflow-hidden border border-slate-800">
                <div
                  className="h-full bg-gradient-to-r from-blue-600 to-cyan-400 transition-all duration-300"
                  style={{ width: `${pct}%` }}
                />
              </div>
              <div className="mt-3 flex flex-wrap gap-4 text-xs text-slate-400">
                <span>Active workers: {progress?.active_workers ?? 0}</span>
                <span>Failed: {progress?.files_failed ?? 0}</span>
                <span>Skipped: {progress?.files_skipped ?? 0}</span>
              </div>
            </div>

            <div className="rounded-xl border border-slate-800 bg-surface-800/60 p-5">
              <h2 className="text-sm font-semibold text-slate-200 mb-3 flex items-center gap-2">
                <Database className="h-4 w-4 text-blue-400" /> Worker matrix
              </h2>
              {(!workers || workers.filter((w) => w.active).length === 0) && (
                <p className="text-sm text-slate-500">No active transfers. Start a job from the Rules tab.</p>
              )}
              <div className="grid sm:grid-cols-2 gap-3">
                {workers
                  .filter((w) => w.active)
                  .map((w) => (
                    <div key={w.worker_id} className="rounded-lg border border-slate-700/80 bg-surface-900/50 p-3">
                      <div className="flex justify-between text-xs text-slate-400 mb-1">
                        <span className="font-mono">worker-{w.worker_id}</span>
                        <span>{(w.percent || 0).toFixed(0)}%</span>
                      </div>
                      <div className="text-sm text-slate-200 truncate font-mono" title={w.key}>
                        {w.key || '—'}
                      </div>
                      <div className="text-[11px] text-slate-500 truncate mt-0.5">
                        {w.source} → {w.target}
                      </div>
                      <div className="mt-2 h-1.5 rounded bg-surface-700 overflow-hidden">
                        <div className="h-full bg-blue-500" style={{ width: `${Math.min(100, w.percent || 0)}%` }} />
                      </div>
                      <div className="mt-1 text-[11px] text-slate-500">
                        {formatBytes(w.bytes_done)} / {formatBytes(w.size_bytes)}
                      </div>
                    </div>
                  ))}
              </div>
            </div>

            <div className="grid lg:grid-cols-2 gap-4">
              <div className="rounded-xl border border-slate-800 bg-surface-800/60 p-5">
                <h2 className="text-sm font-semibold mb-3">Console log</h2>
                <div className="h-64 overflow-y-auto rounded-lg bg-black/40 border border-slate-800 font-mono text-[11px] p-3 space-y-1">
                  {logs.length === 0 && <div className="text-slate-600">Waiting for events…</div>}
                  {logs.map((l, i) => (
                    <div key={i} className="text-slate-300">
                      <span className="text-slate-600">{new Date(l.t).toLocaleTimeString()} </span>
                      {l.m}
                    </div>
                  ))}
                  <div ref={logEndRef} />
                </div>
              </div>

              <div className="rounded-xl border border-slate-800 bg-surface-800/60 p-5">
                <h2 className="text-sm font-semibold mb-3">Recent jobs</h2>
                <div className="space-y-2 max-h-64 overflow-y-auto">
                  {(jobs || []).slice(0, 12).map((j) => (
                    <div key={j.id} className="flex items-center justify-between gap-2 text-sm rounded-lg border border-slate-800 px-3 py-2 bg-surface-900/40">
                      <div className="min-w-0">
                        <div className="font-mono text-xs text-slate-400">{j.id.slice(0, 8)}</div>
                        <div className="text-xs text-slate-500">
                          {formatBytes(j.bytes_transferred)} · {j.files_transferred} files
                        </div>
                      </div>
                      <div className="flex items-center gap-2 shrink-0">
                        {['queued', 'active', 'dry_running'].includes(j.status) && (
                          <button
                            type="button"
                            disabled={busy}
                            onClick={() => cancelJob(j.id)}
                            className="inline-flex items-center gap-1 text-[11px] px-2 py-1 rounded border border-red-500/40 text-red-300 hover:bg-red-950/40"
                            title="Cancel job"
                          >
                            <Square className="h-3 w-3" /> Cancel
                          </button>
                        )}
                        <StatusPill status={j.status} />
                      </div>
                    </div>
                  ))}
                  {jobs?.length === 0 && <p className="text-sm text-slate-500">No jobs yet.</p>}
                </div>
              </div>
            </div>
          </section>
        )}

        {tab === 'providers' && (
          <section className="grid lg:grid-cols-5 gap-4">
            <form onSubmit={saveProvider} className="lg:col-span-2 rounded-xl border border-slate-800 bg-surface-800/60 p-5 space-y-3">
              <h2 className="text-sm font-semibold flex items-center gap-2">
                <Plus className="h-4 w-4" /> Add provider
              </h2>
              <label className="block text-xs text-slate-400">
                Known provider
                <select
                  className="mt-1 w-full rounded-md bg-surface-900 border border-slate-700 px-3 py-2 text-sm text-slate-100 focus:outline-none focus:ring-1 focus:ring-blue-500"
                  value={providerPreset}
                  onChange={(e) => applyProviderPreset(e.target.value)}
                >
                  {KNOWN_PROVIDERS.map((p) => (
                    <option key={p.id} value={p.id}>{p.label}</option>
                  ))}
                </select>
              </label>
              <p className="text-[11px] text-slate-500 leading-relaxed -mt-1">
                {findKnownProvider(providerPreset).hint}
              </p>
              <label className="block text-xs text-slate-400">
                Display name
                <input
                  className="mt-1 w-full rounded-md bg-surface-900 border border-slate-700 px-3 py-2 text-sm text-slate-100 focus:outline-none focus:ring-1 focus:ring-blue-500"
                  value={providerForm.name}
                  onChange={(e) => setProviderForm({ ...providerForm, name: e.target.value })}
                  required
                  placeholder="My production R2"
                />
              </label>
              <label className="block text-xs text-slate-400">
                Type key
                <input
                  className="mt-1 w-full rounded-md bg-surface-900 border border-slate-700 px-3 py-2 text-sm text-slate-100 focus:outline-none focus:ring-1 focus:ring-blue-500 font-mono"
                  value={providerForm.provider_type}
                  onChange={(e) => {
                    setProviderPreset('manual')
                    setProviderForm({ ...providerForm, provider_type: e.target.value })
                  }}
                  required
                  placeholder="aws, r2, minio, …"
                />
              </label>
              <label className="block text-xs text-slate-400">
                Endpoint {providerForm.provider_type === 'aws' ? '(leave empty for AWS)' : ''}
                <input
                  className="mt-1 w-full rounded-md bg-surface-900 border border-slate-700 px-3 py-2 text-sm text-slate-100 focus:outline-none focus:ring-1 focus:ring-blue-500 font-mono"
                  value={providerForm.endpoint}
                  onChange={(e) => setProviderForm({ ...providerForm, endpoint: e.target.value })}
                  placeholder={providerForm.provider_type === 'aws' ? 'empty = default AWS endpoint' : 'https://…'}
                />
              </label>
              <label className="block text-xs text-slate-400">
                Region
                <input
                  className="mt-1 w-full rounded-md bg-surface-900 border border-slate-700 px-3 py-2 text-sm text-slate-100 focus:outline-none focus:ring-1 focus:ring-blue-500 font-mono"
                  value={providerForm.region}
                  onChange={(e) => setProviderForm({ ...providerForm, region: e.target.value })}
                  required
                  placeholder="us-east-1"
                />
              </label>
              <label className="block text-xs text-slate-400">
                Access key
                <input
                  className="mt-1 w-full rounded-md bg-surface-900 border border-slate-700 px-3 py-2 text-sm text-slate-100 focus:outline-none focus:ring-1 focus:ring-blue-500"
                  value={providerForm.access_key_id}
                  onChange={(e) => setProviderForm({ ...providerForm, access_key_id: e.target.value })}
                  required
                  autoComplete="off"
                />
              </label>
              <label className="block text-xs text-slate-400">
                Secret key
                <input
                  className="mt-1 w-full rounded-md bg-surface-900 border border-slate-700 px-3 py-2 text-sm text-slate-100 focus:outline-none focus:ring-1 focus:ring-blue-500"
                  type="password"
                  value={providerForm.secret_access_key}
                  onChange={(e) => setProviderForm({ ...providerForm, secret_access_key: e.target.value })}
                  required
                  autoComplete="new-password"
                />
              </label>
              <div className="flex gap-2 pt-1">
                <button
                  type="button"
                  disabled={busy}
                  onClick={testProvider}
                  className="flex-1 inline-flex justify-center items-center gap-1.5 rounded-md border border-slate-600 px-3 py-2 text-sm hover:bg-surface-700"
                >
                  {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : <Cloud className="h-4 w-4" />}
                  Test connection
                </button>
                <button
                  type="submit"
                  disabled={busy}
                  className="flex-1 inline-flex justify-center items-center gap-1.5 rounded-md bg-blue-600 hover:bg-blue-500 px-3 py-2 text-sm font-medium"
                >
                  Save
                </button>
              </div>
              {testResult && (
                <div className={`text-xs rounded-md px-3 py-2 border ${testResult.ok ? 'border-emerald-500/40 text-emerald-300 bg-emerald-950/30' : 'border-red-500/40 text-red-300 bg-red-950/30'}`}>
                  {testResult.ok ? (
                    <span className="inline-flex items-center gap-1">
                      <CheckCircle2 className="h-3.5 w-3.5" /> Connected · {testResult.latency_ms} ms · {testResult.bucket_count} buckets
                    </span>
                  ) : (
                    <span>Failed: {testResult.error}</span>
                  )}
                </div>
              )}
            </form>

            <div className="lg:col-span-3 grid sm:grid-cols-2 gap-3 content-start">
              {providers.map((p) => (
                <div key={p.id} className="rounded-xl border border-slate-800 bg-surface-800/60 p-4 flex flex-col gap-2">
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <h3 className="font-medium text-slate-100">{p.name}</h3>
                      <p className="text-xs text-slate-500 uppercase tracking-wide">{p.provider_type}</p>
                    </div>
                    <button type="button" onClick={() => deleteProvider(p.id)} className="text-slate-500 hover:text-red-400">
                      <Trash2 className="h-4 w-4" />
                    </button>
                  </div>
                  <div className="text-xs text-slate-400 font-mono break-all">{p.endpoint || 'default AWS endpoint'}</div>
                  <div className="text-xs text-slate-500">Region: {p.region}</div>
                  <div className="text-xs text-slate-500">Key: {p.access_key_id}</div>
                </div>
              ))}
              {providers.length === 0 && (
                <p className="text-sm text-slate-500 col-span-2">No providers yet. Add credentials to start building pipelines.</p>
              )}
            </div>
          </section>
        )}

        {tab === 'rules' && (
          <section className="grid lg:grid-cols-2 gap-4">
            <form onSubmit={saveRule} className="rounded-xl border border-slate-800 bg-surface-800/60 p-5 space-y-3">
              <h2 className="text-sm font-semibold">Sync pipeline</h2>
              <label className="block text-xs text-slate-400">
                Name
                <input className="field" value={ruleForm.name} onChange={(e) => setRuleForm({ ...ruleForm, name: e.target.value })} required />
              </label>
              <div className="grid grid-cols-2 gap-3">
                <label className="block text-xs text-slate-400">
                  Source provider
                  <select className="field" value={ruleForm.source_provider_id} onChange={(e) => setRuleForm({ ...ruleForm, source_provider_id: e.target.value })} required>
                    <option value="">Select…</option>
                    {providers.map((p) => (
                      <option key={p.id} value={p.id}>{p.name}</option>
                    ))}
                  </select>
                </label>
                <label className="block text-xs text-slate-400">
                  Target provider
                  <select className="field" value={ruleForm.target_provider_id} onChange={(e) => setRuleForm({ ...ruleForm, target_provider_id: e.target.value })} required>
                    <option value="">Select…</option>
                    {providers.map((p) => (
                      <option key={p.id} value={p.id}>{p.name}</option>
                    ))}
                  </select>
                </label>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <label className="block text-xs text-slate-400">
                  Source bucket
                  <input className="field" value={ruleForm.source_bucket} onChange={(e) => setRuleForm({ ...ruleForm, source_bucket: e.target.value })} required />
                </label>
                <label className="block text-xs text-slate-400">
                  Target bucket
                  <input className="field" value={ruleForm.target_bucket} onChange={(e) => setRuleForm({ ...ruleForm, target_bucket: e.target.value })} required />
                </label>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <label className="block text-xs text-slate-400">
                  Source prefix
                  <input className="field" value={ruleForm.source_prefix} onChange={(e) => setRuleForm({ ...ruleForm, source_prefix: e.target.value })} />
                </label>
                <label className="block text-xs text-slate-400">
                  Target prefix
                  <input className="field" value={ruleForm.target_prefix} onChange={(e) => setRuleForm({ ...ruleForm, target_prefix: e.target.value })} />
                </label>
              </div>
              <label className="block text-xs text-slate-400">
                Extra targets (same provider; semicolon-separated <code className="text-slate-500">bucket</code> or <code className="text-slate-500">bucket:prefix</code>)
                <input
                  className="field"
                  placeholder="backup-bucket;archive:cold/"
                  value={ruleForm.extra_targets}
                  onChange={(e) => setRuleForm({ ...ruleForm, extra_targets: e.target.value })}
                />
              </label>
              <div className="grid grid-cols-2 gap-3">
                <label className="block text-xs text-slate-400">
                  Include (e.g. .png;.jpg)
                  <input className="field" value={ruleForm.include_patterns} onChange={(e) => setRuleForm({ ...ruleForm, include_patterns: e.target.value })} />
                </label>
                <label className="block text-xs text-slate-400">
                  Exclude
                  <input className="field" value={ruleForm.exclude_patterns} onChange={(e) => setRuleForm({ ...ruleForm, exclude_patterns: e.target.value })} />
                </label>
              </div>
              <div className="grid grid-cols-3 gap-3">
                <label className="block text-xs text-slate-400">
                  Min size (bytes)
                  <input className="field" type="number" value={ruleForm.min_size_bytes} onChange={(e) => setRuleForm({ ...ruleForm, min_size_bytes: e.target.value })} />
                </label>
                <label className="block text-xs text-slate-400">
                  Max size
                  <input className="field" type="number" value={ruleForm.max_size_bytes} onChange={(e) => setRuleForm({ ...ruleForm, max_size_bytes: e.target.value })} />
                </label>
                <label className="block text-xs text-slate-400">
                  Modified after
                  <input className="field" type="datetime-local" value={ruleForm.modified_after} onChange={(e) => setRuleForm({ ...ruleForm, modified_after: e.target.value ? new Date(e.target.value).toISOString() : '' })} />
                </label>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <label className="block text-xs text-slate-400">
                  Concurrency (1–32)
                  <input className="field" type="number" min={1} max={32} value={ruleForm.concurrency_limit} onChange={(e) => setRuleForm({ ...ruleForm, concurrency_limit: e.target.value })} />
                </label>
                <label className="block text-xs text-slate-400">
                  Bandwidth limit (kbps, 0=∞)
                  <input className="field" type="number" min={0} value={ruleForm.bandwidth_limit_kbps} onChange={(e) => setRuleForm({ ...ruleForm, bandwidth_limit_kbps: e.target.value })} />
                </label>
              </div>
              <label className="block text-xs text-slate-400">
                Compare mode
                <select className="field" value={ruleForm.compare_mode} onChange={(e) => setRuleForm({ ...ruleForm, compare_mode: e.target.value })}>
                  <option value="etag">Size + ETag (default)</option>
                  <option value="size">Size only</option>
                </select>
              </label>
              <label className="flex items-center gap-2 text-sm text-slate-300">
                <input type="checkbox" checked={ruleForm.delete_on_target} onChange={(e) => setRuleForm({ ...ruleForm, delete_on_target: e.target.checked })} />
                Strict mirror (delete on target)
              </label>
              <div className="grid grid-cols-2 gap-3">
                <label className="block text-xs text-slate-400 col-span-2">
                  Schedule cron (5-field, e.g. */15 * * * *)
                  <input className="field" placeholder="leave empty for manual only" value={ruleForm.schedule_cron} onChange={(e) => setRuleForm({ ...ruleForm, schedule_cron: e.target.value })} />
                </label>
                <label className="flex items-center gap-2 text-sm text-slate-300 col-span-2">
                  <input type="checkbox" checked={ruleForm.schedule_enabled} onChange={(e) => setRuleForm({ ...ruleForm, schedule_enabled: e.target.checked })} />
                  Enable schedule
                </label>
              </div>
              <button type="submit" disabled={busy} className="w-full rounded-md bg-blue-600 hover:bg-blue-500 py-2 text-sm font-medium">
                Save rule
              </button>
            </form>

            <div className="space-y-4">
              <div className="rounded-xl border border-slate-800 bg-surface-800/60 p-5">
                <h2 className="text-sm font-semibold mb-3">Saved rules</h2>
                <div className="space-y-2">
                  {rules.map((r) => (
                    <div key={r.id} className="rounded-lg border border-slate-800 p-3 bg-surface-900/40">
                      <div className="flex items-center justify-between gap-2">
                        <div>
                          <div className="font-medium text-sm">{r.name}</div>
                          <div className="text-[11px] text-slate-500 font-mono">
                            {r.source_bucket}/{r.source_prefix || ''} → {r.target_bucket}/{r.target_prefix || ''}
                          </div>
                          <div className="text-[11px] text-slate-500 mt-0.5">
                            {r.delete_on_target ? 'Mirror' : 'Safe sync'} · concurrency {r.concurrency_limit}
                            {r.schedule_enabled && r.schedule_cron
                              ? ` · cron ${r.schedule_cron}${r.next_run_at ? ` · next ${new Date(r.next_run_at).toLocaleString()}` : ''}`
                              : ''}
                          </div>
                        </div>
                        <div className="flex flex-col gap-1">
                          <button type="button" onClick={() => runDryRun(r.id)} className="text-xs px-2 py-1 rounded border border-slate-600 hover:bg-surface-700">
                            Dry-run
                          </button>
                          <button type="button" onClick={() => startJob(r.id)} className="text-xs px-2 py-1 rounded bg-emerald-600/80 hover:bg-emerald-500 inline-flex items-center gap-1 justify-center">
                            <Play className="h-3 w-3" /> Start
                          </button>
                        </div>
                      </div>
                    </div>
                  ))}
                  {rules.length === 0 && <p className="text-sm text-slate-500">No rules configured.</p>}
                </div>
              </div>

              <div className="rounded-xl border border-slate-800 bg-surface-800/60 p-5">
                <h2 className="text-sm font-semibold mb-3">Dry-run matrix {selectedRuleId && <span className="text-slate-500 font-normal">· {selectedRuleId.slice(0, 8)}</span>}</h2>
                {!dryRun && <p className="text-sm text-slate-500">Run a dry-run to preview adds, modifies, deletes, and skips.</p>}
                {dryRun && (
                  <>
                    <div className="grid grid-cols-4 gap-2 mb-3 text-center text-xs">
                      <Metric label="Add" value={dryRun.add_count} color="text-emerald-400" />
                      <Metric label="Modify" value={dryRun.modify_count} color="text-amber-400" />
                      <Metric label="Delete" value={dryRun.delete_count} color="text-red-400" />
                      <Metric label="Skip" value={dryRun.skip_count} color="text-slate-400" />
                    </div>
                    <p className="text-xs text-slate-500 mb-2">Bytes to sync: {formatBytes(dryRun.total_bytes_to_sync)}</p>
                    <div className="max-h-64 overflow-y-auto text-[11px] font-mono space-y-1">
                      {(dryRun.items || []).slice(0, 200).map((it, i) => (
                        <div key={i} className="flex gap-2 border-b border-slate-800/80 py-1">
                          <span className={`uppercase w-14 shrink-0 ${
                            it.action === 'add' ? 'text-emerald-400' :
                            it.action === 'modify' ? 'text-amber-400' :
                            it.action === 'delete' ? 'text-red-400' : 'text-slate-500'
                          }`}>{it.action}</span>
                          <span className="text-slate-300 truncate">{it.source_key || it.target_key}</span>
                          {it.destination && <span className="text-slate-500 shrink-0">→ {it.destination}</span>}
                          <span className="text-slate-600 ml-auto shrink-0">{formatBytes(it.size)}</span>
                        </div>
                      ))}
                    </div>
                  </>
                )}
              </div>
            </div>
          </section>
        )}
      </main>

      <footer className="border-t border-slate-800/80 py-3 text-center text-[11px] text-slate-600">
        Gantry · GPL-3.0 · stream-only S3 sync · ghcr.io/arianar/gantry
      </footer>

      <style>{`
        .field {
          display: block;
          width: 100%;
          margin-top: 0.25rem;
          border-radius: 0.375rem;
          background: #0b0f14;
          border: 1px solid #334155;
          padding: 0.5rem 0.75rem;
          font-size: 0.875rem;
          color: #f1f5f9;
        }
        .field:focus {
          outline: none;
          box-shadow: 0 0 0 1px #3b82f6;
        }
      `}</style>
    </div>
  )
}

function StatCard({ icon: Icon, label, value, sub }) {
  return (
    <div className="rounded-xl border border-slate-800 bg-surface-800/60 p-4">
      <div className="flex items-center gap-2 text-xs text-slate-400 mb-1">
        <Icon className="h-3.5 w-3.5" /> {label}
      </div>
      <div className="text-xl font-semibold tracking-tight">{value}</div>
      {sub && <div className="text-xs text-slate-500 mt-0.5">{sub}</div>}
    </div>
  )
}

function Metric({ label, value, color }) {
  return (
    <div className="rounded-lg bg-surface-900/60 border border-slate-800 py-2">
      <div className={`text-lg font-semibold ${color}`}>{value}</div>
      <div className="text-slate-500">{label}</div>
    </div>
  )
}

function StatusPill({ status }) {
  const colors = {
    active: 'bg-blue-500/20 text-blue-300 border-blue-500/40',
    queued: 'bg-slate-500/20 text-slate-300 border-slate-500/40',
    completed: 'bg-emerald-500/20 text-emerald-300 border-emerald-500/40',
    failed: 'bg-red-500/20 text-red-300 border-red-500/40',
    cancelled: 'bg-amber-500/20 text-amber-300 border-amber-500/40',
  }
  return (
    <span className={`text-[10px] uppercase tracking-wide px-2 py-0.5 rounded border ${colors[status] || colors.queued}`}>
      {status}
    </span>
  )
}
