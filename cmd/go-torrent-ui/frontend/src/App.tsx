import { useState, useEffect, useCallback } from 'react';
import { 
  Sun, Moon, Plus, Link2, Trash2, Download, Users, 
  AlertCircle, CheckCircle2, Loader2, File, FolderOpen, 
  Clipboard, ChevronDown, Pause, Play,
  Zap, Clock, HardDrive, Sparkles, X, Globe, Copy, Search, RefreshCw
} from 'lucide-react';
import './style.css';

interface TorrentStatus {
  id: string;
  name: string;
  progress: number;
  downSpeed: number;
  upSpeed: number;
  peers: number;
  seeds: number;
  size: number;
  downloaded: number;
  status: 'downloading' | 'paused' | 'completed' | 'error' | 'starting';
  error?: string;
}

interface DHTStatus {
  running: boolean;
  nodeId: string;
  port: number;
  nodeCount: number;
}

interface DHTNodeInfo {
  id: string;
  address: string;
  lastSeen: string;
}

declare global {
  interface Window {
    go: {
      main: {
        App: {
          GetTorrents(): Promise<TorrentStatus[]>;
          AddMagnet(magnetLink: string, outputPath: string): Promise<string>;
          AddTorrentFile(filePath: string, outputPath: string): Promise<string>;
          RemoveTorrent(id: string): Promise<void>;
          PauseTorrent(id: string): Promise<void>;
          ResumeTorrent(id: string): Promise<void>;
          SelectTorrentFile(): Promise<string>;
          SelectOutputFolder(): Promise<string>;
          GetDHTStatus(): Promise<DHTStatus>;
          GetDHTNodes(): Promise<DHTNodeInfo[]>;
        };
      };
    };
  }
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function formatSpeed(bytesPerSec: number): string {
  return formatBytes(bytesPerSec) + '/s';
}

function formatETA(bytes: number, speed: number): string {
  if (speed <= 0) return '∞';
  const seconds = bytes / speed;
  const mins = Math.floor(seconds / 60);
  if (mins >= 60) {
    const hours = Math.floor(mins / 60);
    return `${hours}h ${mins % 60}m`;
  }
  const secs = Math.floor(seconds % 60);
  return `${mins}:${secs.toString().padStart(2, '0')}`;
}

function App() {
  const [darkMode, setDarkMode] = useState(false);
  const [torrents, setTorrents] = useState<TorrentStatus[]>([]);
  const [showAddModal, setShowAddModal] = useState(false);
  const [magnetInput, setMagnetInput] = useState('');
  const [torrentFile, setTorrentFile] = useState('');
  const [outputPath, setOutputPath] = useState('./downloads');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [dhtStatus, setDhtStatus] = useState<DHTStatus>({ running: false, nodeId: '', port: 0, nodeCount: 0 });
  const [showDHTPanel, setShowDHTPanel] = useState(false);
  const [dhtNodes, setDhtNodes] = useState<DHTNodeInfo[]>([]);
  const [dhtNodesLoading, setDhtNodesLoading] = useState(false);
  const [nodeFilter, setNodeFilter] = useState('');

  useEffect(() => {
    document.documentElement.classList.toggle('dark', darkMode);
  }, [darkMode]);

  const fetchTorrents = useCallback(async () => {
    try {
      if (window.go?.main?.App?.GetTorrents) {
        const result = await window.go.main.App.GetTorrents();
        setTorrents(result || []);
      }
    } catch (e) {
      console.error('Failed to fetch torrents:', e);
    }
  }, []);

  useEffect(() => {
    fetchTorrents();
    const interval = setInterval(fetchTorrents, 1000);
    return () => clearInterval(interval);
  }, [fetchTorrents]);

  // Fetch DHT status periodically
  useEffect(() => {
    const fetchDHT = async () => {
      try {
        if (window.go?.main?.App?.GetDHTStatus) {
          const status = await window.go.main.App.GetDHTStatus();
          setDhtStatus(status);
        }
      } catch (e) {
        console.error('Failed to fetch DHT status:', e);
      }
    };
    fetchDHT();
    const interval = setInterval(fetchDHT, 3000);
    return () => clearInterval(interval);
  }, []);

  const handleShowDHTNodes = async () => {
    if (showDHTPanel) {
      setShowDHTPanel(false);
      return;
    }
    setDhtNodesLoading(true);
    setShowDHTPanel(true);
    try {
      if (window.go?.main?.App?.GetDHTNodes) {
        const nodes = await window.go.main.App.GetDHTNodes();
        setDhtNodes(nodes || []);
      }
    } catch (e) {
      console.error('Failed to fetch DHT nodes:', e);
    } finally {
      setDhtNodesLoading(false);
    }
  };

  const refreshDHTNodes = async () => {
    setDhtNodesLoading(true);
    try {
      if (window.go?.main?.App?.GetDHTNodes) {
        const nodes = await window.go.main.App.GetDHTNodes();
        setDhtNodes(nodes || []);
      }
    } catch (e) {
      console.error('Failed to fetch DHT nodes:', e);
    } finally {
      setDhtNodesLoading(false);
    }
  };

  const handleSelectTorrentFile = async () => {
    try {
      const path = await window.go.main.App.SelectTorrentFile();
      if (path) {
        setTorrentFile(path);
        setMagnetInput('');
      }
    } catch (e) {
      console.error('Failed to select file:', e);
    }
  };

  const handleSelectOutputFolder = async () => {
    try {
      const path = await window.go.main.App.SelectOutputFolder();
      if (path) setOutputPath(path);
    } catch (e) {
      console.error('Failed to select folder:', e);
    }
  };

  const handlePasteMagnet = async () => {
    try {
      const text = await navigator.clipboard.readText();
      if (text.startsWith('magnet:')) {
        setMagnetInput(text);
        setTorrentFile('');
        setShowAddModal(true);
      }
    } catch (e) {
      console.error('Failed to paste:', e);
    }
  };

  const handleAdd = async () => {
    if (!magnetInput.trim() && !torrentFile) return;
    setIsLoading(true);
    setError(null);
    try {
      if (torrentFile) {
        await window.go.main.App.AddTorrentFile(torrentFile, outputPath);
      } else {
        await window.go.main.App.AddMagnet(magnetInput, outputPath);
      }
      setMagnetInput('');
      setTorrentFile('');
      setShowAddModal(false);
      fetchTorrents();
    } catch (e: any) {
      setError(e.message || 'Failed to add torrent');
    } finally {
      setIsLoading(false);
    }
  };

  const handleRemove = async (id: string) => {
    try {
      await window.go.main.App.RemoveTorrent(id);
      if (selectedId === id) setSelectedId(null);
      fetchTorrents();
    } catch (e) {
      console.error('Failed to remove:', e);
    }
  };

  const handlePause = async (id: string) => {
    try {
      await window.go.main.App.PauseTorrent(id);
      fetchTorrents();
    } catch (e) {
      console.error('Failed to pause:', e);
    }
  };

  const handleResume = async (id: string) => {
    try {
      await window.go.main.App.ResumeTorrent(id);
      fetchTorrents();
    } catch (e) {
      console.error('Failed to resume:', e);
    }
  };

  const activeTorrents = torrents.filter(t => t.status !== 'completed');
  const completedTorrents = torrents.filter(t => t.status === 'completed');
  const totalDown = torrents.reduce((a, t) => a + (t.downSpeed || 0), 0);

  return (
    <div className="h-screen flex flex-col overflow-hidden" style={{ padding: '24px', gap: '20px' }}>
      {/* Header */}
      <header className="glass h-16 flex items-center justify-between rounded-2xl border border-[var(--border)] shadow-[var(--shadow-sm)]" style={{ padding: '0 24px' }}>
        <div className="flex items-center" style={{ gap: '20px' }}>
          {/* Logo */}
          <div className="flex items-center" style={{ gap: '12px' }}>
            <div 
              className="w-10 h-10 rounded-xl flex items-center justify-center shadow-[var(--shadow-glow)]"
              style={{ 
                background: 'linear-gradient(135deg, #10b981 0%, #059669 50%, #047857 100%)',
              }}
            >
              <Sparkles className="w-5 h-5 text-white" />
            </div>
            <span className="font-black text-lg tracking-tight" style={{ color: 'var(--text)' }}>
              GoTorrent
            </span>
          </div>

          <div className="h-6 w-px bg-[var(--border)]" />

          {/* Actions */}
          <div className="flex" style={{ gap: '12px' }}>
            <button
              onClick={() => setShowAddModal(true)}
              className="rounded-lg font-semibold text-sm flex items-center transition-all duration-200 hover:scale-[1.02] active:scale-[0.98]"
              style={{ 
                padding: '10px 20px',
                gap: '10px',
                background: 'linear-gradient(135deg, var(--accent) 0%, #059669 100%)',
                color: 'white',
                boxShadow: '0 2px 8px var(--accent-glow)'
              }}
            >
              <Plus className="w-4 h-4" strokeWidth={2.5} />
              Add
            </button>
            <button
              onClick={handlePasteMagnet}
              className="rounded-lg font-medium text-sm flex items-center transition-all duration-200 hover:bg-[var(--border-strong)] border border-[var(--border)]"
              style={{ padding: '10px 20px', gap: '10px', color: 'var(--text-secondary)' }}
            >
              <Clipboard className="w-4 h-4" />
              Paste
            </button>
          </div>
        </div>

        <div className="flex items-center" style={{ gap: '12px' }}>
          {/* Speed Display */}
          <div 
            className="flex items-center rounded-lg border border-[var(--border)]"
            style={{ padding: '8px 16px', fontFamily: "'JetBrains Mono', monospace" }}
          >
            <span className="flex items-center text-xs font-medium" style={{ gap: '8px', color: 'var(--accent)' }}>
              <Download className="w-3.5 h-3.5" />
              {formatSpeed(totalDown)}
            </span>
          </div>

          {/* Theme Toggle */}
          <button
            onClick={() => setDarkMode(!darkMode)}
            className="w-9 h-9 rounded-lg flex items-center justify-center transition-all duration-200 hover:bg-[var(--border-strong)] border border-transparent hover:border-[var(--border)]"
          >
            {darkMode ? (
              <Sun className="w-4 h-4" style={{ color: 'var(--text-secondary)' }} />
            ) : (
              <Moon className="w-4 h-4" style={{ color: 'var(--text-secondary)' }} />
            )}
          </button>
        </div>
      </header>

      {/* Main Content */}
      <main className="flex-1 overflow-auto flex flex-col">
        {torrents.length === 0 ? (
          <div className="flex-1 flex items-center justify-center">
            <div className="text-center animate-fade-in flex flex-col items-center">
              {/* Animated Icon */}
              <div className="relative w-28 h-28 mx-auto" style={{ marginBottom: '32px' }}>
                <div 
                  className="absolute inset-0 rounded-3xl animate-pulse-glow"
                  style={{ background: 'linear-gradient(135deg, var(--accent) 0%, #059669 100%)' }}
                />
                <div 
                  className="relative w-full h-full rounded-3xl flex items-center justify-center animate-float"
                  style={{ 
                    background: 'linear-gradient(135deg, var(--accent) 0%, #059669 50%, #047857 100%)',
                  }}
                >
                  <Download className="w-12 h-12 text-white" strokeWidth={1.5} />
                </div>
              </div>

              <h2 
                className="text-2xl font-black tracking-tight"
                style={{ color: 'var(--text)', marginBottom: '12px' }}
              >
                Ready to Download
              </h2>
              <p className="text-sm max-w-xs mx-auto" style={{ color: 'var(--text-muted)', marginBottom: '32px' }}>
                Add a torrent file or paste a magnet link to start downloading
              </p>

              <div className="flex justify-center" style={{ gap: '20px' }}>
                <button
                  onClick={() => setShowAddModal(true)}
                  className="rounded-xl font-semibold text-sm flex items-center transition-all duration-200 hover:scale-[1.02] active:scale-[0.98]"
                  style={{ 
                    padding: '14px 28px',
                    gap: '12px',
                    background: 'linear-gradient(135deg, var(--accent) 0%, #059669 100%)',
                    color: 'white',
                    boxShadow: '0 4px 16px var(--accent-glow)'
                  }}
                >
                  <Plus className="w-5 h-5" strokeWidth={2.5} />
                  Add Torrent
                </button>
                <button
                  onClick={handlePasteMagnet}
                  className="glass rounded-xl font-semibold text-sm flex items-center border border-[var(--border)] transition-all duration-200 hover:border-[var(--border-strong)] hover:shadow-[var(--shadow-md)]"
                  style={{ padding: '14px 28px', gap: '12px', color: 'var(--text-secondary)' }}
                >
                  <Link2 className="w-5 h-5" />
                  Paste Magnet
                </button>
              </div>
            </div>
          </div>
        ) : (
          <div className="w-full flex flex-col" style={{ gap: '16px' }}>
            {activeTorrents.length > 0 && (
              <TorrentSection 
                title="Active" 
                count={activeTorrents.length}
                accent
              >
                {activeTorrents.map((t, i) => (
                  <TorrentCard 
                    key={t.id} 
                    torrent={t} 
                    selected={selectedId === t.id}
                    onSelect={() => setSelectedId(t.id)}
                    onRemove={() => handleRemove(t.id)}
                    onPause={() => handlePause(t.id)}
                    onResume={() => handleResume(t.id)}
                    delay={i * 50}
                  />
                ))}
              </TorrentSection>
            )}
            {completedTorrents.length > 0 && (
              <TorrentSection 
                title="Completed" 
                count={completedTorrents.length}
              >
                {completedTorrents.map((t, i) => (
                  <TorrentCard 
                    key={t.id} 
                    torrent={t}
                    selected={selectedId === t.id}
                    onSelect={() => setSelectedId(t.id)}
                    onRemove={() => handleRemove(t.id)}
                    onPause={() => handlePause(t.id)}
                    onResume={() => handleResume(t.id)}
                    delay={i * 50}
                  />
                ))}
              </TorrentSection>
            )}
          </div>
        )}
      </main>

      {/* Status Bar */}
      <footer 
        className="glass h-10 flex items-center justify-between rounded-xl border border-[var(--border)] text-xs"
        style={{ padding: '0 24px', fontFamily: "'JetBrains Mono', monospace" }}
      >
        <div className="flex items-center" style={{ gap: '20px' }}>
          <button
            onClick={handleShowDHTNodes}
            className="flex items-center font-medium transition-colors hover:opacity-80"
            style={{ gap: '8px', color: dhtStatus.running ? 'var(--accent)' : 'var(--text-muted)', background: 'none', border: 'none', cursor: 'pointer', padding: 0 }}
            title={dhtStatus.running ? `DHT running on port ${dhtStatus.port}` : 'DHT offline'}
          >
            <Globe className={`w-3.5 h-3.5 ${dhtStatus.running ? 'animate-pulse' : ''}`} />
            DHT
            {dhtStatus.running && (
              <span className="rounded-md text-[10px] font-bold" style={{ padding: '1px 6px', background: 'var(--accent-glow)', color: 'var(--accent)' }}>
                {dhtStatus.nodeCount}
              </span>
            )}
          </button>
          <span style={{ color: 'var(--text-muted)' }}>
            {torrents.length} torrent{torrents.length !== 1 ? 's' : ''}
          </span>
        </div>
        <span className="flex items-center" style={{ gap: '8px', color: 'var(--text-muted)' }}>
          <span style={{ color: 'var(--accent)' }}>↓</span>
          {formatSpeed(totalDown)}
        </span>
      </footer>

      {/* DHT Nodes Panel */}
      {showDHTPanel && (
        <div 
          className="fixed inset-0 z-50 flex items-center justify-center"
          style={{ padding: '16px' }}
          onClick={() => setShowDHTPanel(false)}
        >
          <div className="absolute inset-0 bg-black/40 backdrop-blur-md" />
          <div 
            className="glass relative w-full max-w-2xl rounded-2xl border border-[var(--border)] shadow-[var(--shadow-lg)] animate-fade-in"
            style={{ background: 'var(--surface-solid)', padding: '24px', maxHeight: '80vh', display: 'flex', flexDirection: 'column' }}
            onClick={e => e.stopPropagation()}
          >
            {/* Header */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '20px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                <div 
                  className="w-9 h-9 rounded-xl"
                  style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', background: dhtStatus.running ? 'var(--accent-glow)' : 'var(--border)', flexShrink: 0 }}
                >
                  <Globe className="w-4 h-4" style={{ color: dhtStatus.running ? 'var(--accent)' : 'var(--text-muted)' }} />
                </div>
                <div>
                  <h2 className="text-lg font-bold" style={{ color: 'var(--text)' }}>DHT Network</h2>
                  <div className="text-[11px]" style={{ color: 'var(--text-muted)', fontFamily: "'JetBrains Mono', monospace" }}>
                    {dhtStatus.running ? `Port ${dhtStatus.port} · ${dhtStatus.nodeCount} nodes` : 'Offline'}
                  </div>
                </div>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexShrink: 0 }}>
                <button
                  onClick={refreshDHTNodes}
                  disabled={dhtNodesLoading}
                  className="w-8 h-8 rounded-lg hover:bg-[var(--border-strong)] transition-colors border border-[var(--border)]"
                  style={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}
                  title="Refresh"
                >
                  <RefreshCw className={`w-4 h-4 ${dhtNodesLoading ? 'animate-spin' : ''}`} style={{ color: 'var(--text-muted)' }} />
                </button>
                <button 
                  onClick={() => setShowDHTPanel(false)}
                  className="w-8 h-8 rounded-lg hover:bg-[var(--border-strong)] transition-colors"
                  style={{ display: 'flex', alignItems: 'center', justifyContent: 'center' }}
                >
                  <X className="w-4 h-4" style={{ color: 'var(--text-muted)' }} />
                </button>
              </div>
            </div>

            {/* Node ID */}
            {dhtStatus.running && (
              <div 
                className="rounded-lg border border-[var(--border)] text-[11px]"
                style={{ display: 'flex', alignItems: 'center', padding: '8px 12px', marginBottom: '16px', fontFamily: "'JetBrains Mono', monospace", color: 'var(--text-muted)' }}
              >
                <span className="font-semibold" style={{ marginRight: '8px', color: 'var(--text-secondary)' }}>Node ID</span>
                <span className="truncate flex-1">{dhtStatus.nodeId}</span>
                <button
                  onClick={() => navigator.clipboard.writeText(dhtStatus.nodeId)}
                  className="ml-2 hover:text-[var(--text)] transition-colors flex-shrink-0"
                  title="Copy"
                >
                  <Copy className="w-3 h-3" />
                </button>
              </div>
            )}

            {/* Search */}
            <div className="relative" style={{ marginBottom: '12px' }}>
              <Search className="absolute w-3.5 h-3.5" style={{ left: '12px', top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
              <input
                type="text"
                value={nodeFilter}
                onChange={e => setNodeFilter(e.target.value)}
                placeholder="Filter by address or ID..."
                className="w-full rounded-lg text-xs border border-[var(--border)] focus:border-[var(--accent)] focus:outline-none transition-colors"
                style={{ height: '36px', padding: '0 12px 0 32px', background: 'var(--bg)', color: 'var(--text)', fontFamily: "'JetBrains Mono', monospace" }}
              />
            </div>

            {/* Nodes Table */}
            <div className="flex-1 overflow-auto rounded-lg border border-[var(--border)]" style={{ minHeight: 0 }}>
              {/* Table Header */}
              <div 
                className="text-[10px] font-bold uppercase tracking-wide sticky top-0 border-b border-[var(--border)]"
                style={{ display: 'flex', alignItems: 'center', padding: '8px 16px', background: 'var(--surface-solid)', color: 'var(--text-muted)' }}
              >
                <span style={{ width: '55%' }}>Address</span>
                <span style={{ width: '30%' }}>Node ID</span>
                <span style={{ width: '15%', textAlign: 'right' }}>Last Seen</span>
              </div>

              {/* Nodes */}
              <div>
                {dhtNodesLoading && dhtNodes.length === 0 ? (
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '40px', color: 'var(--text-muted)' }}>
                    <Loader2 className="w-5 h-5 animate-spin" />
                  </div>
                ) : (() => {
                  const filtered = dhtNodes.filter(n => 
                    !nodeFilter || n.address.includes(nodeFilter) || n.id.includes(nodeFilter.toLowerCase())
                  );
                  return filtered.length === 0 ? (
                    <div className="text-center text-xs" style={{ padding: '40px', color: 'var(--text-muted)' }}>
                      {dhtNodes.length === 0 ? 'No nodes discovered yet' : 'No matching nodes'}
                    </div>
                  ) : (
                    filtered.map((node, i) => (
                      <div
                        key={node.id + node.address}
                        className="text-[11px] border-b last:border-b-0 border-[var(--border)] hover:bg-[var(--border)] transition-colors"
                        style={{ display: 'flex', alignItems: 'center', padding: '6px 16px', fontFamily: "'JetBrains Mono', monospace", animationDelay: `${i * 10}ms` }}
                      >
                        <span className="truncate" style={{ width: '55%', color: 'var(--text)' }}>{node.address}</span>
                        <span className="truncate" style={{ width: '30%', color: 'var(--text-muted)' }}>{node.id.slice(0, 16)}…</span>
                        <span style={{ width: '15%', textAlign: 'right', color: 'var(--text-muted)' }}>
                          {(() => {
                            const d = new Date(node.lastSeen);
                            const now = new Date();
                            const diffSec = Math.floor((now.getTime() - d.getTime()) / 1000);
                            if (diffSec < 60) return `${diffSec}s ago`;
                            const diffMin = Math.floor(diffSec / 60);
                            if (diffMin < 60) return `${diffMin}m ago`;
                            return `${Math.floor(diffMin / 60)}h ago`;
                          })()}
                        </span>
                      </div>
                    ))
                  );
                })()}
              </div>
            </div>

            {/* Footer */}
            <div className="text-[11px]" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginTop: '12px', color: 'var(--text-muted)' }}>
              <span>{dhtNodes.length} nodes total</span>
              {dhtStatus.running && (
                <span style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                  <span className="w-1.5 h-1.5 rounded-full" style={{ background: 'var(--accent)' }} />
                  Connected
                </span>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Modal */}
      {showAddModal && (
        <div 
          className="fixed inset-0 z-50 flex items-center justify-center"
          style={{ padding: '16px' }}
          onClick={() => setShowAddModal(false)}
        >
          <div className="absolute inset-0 bg-black/40 backdrop-blur-md" />
          <div 
            className="glass relative w-full max-w-md rounded-2xl border border-[var(--border)] shadow-[var(--shadow-lg)] animate-fade-in"
            style={{ background: 'var(--surface-solid)', padding: '24px' }}
            onClick={e => e.stopPropagation()}
          >
            {/* Header */}
            <div className="flex items-center justify-between" style={{ marginBottom: '24px' }}>
              <h2 className="text-lg font-bold" style={{ color: 'var(--text)' }}>Add Torrent</h2>
              <button 
                onClick={() => setShowAddModal(false)}
                className="w-8 h-8 rounded-lg flex items-center justify-center hover:bg-[var(--border-strong)] transition-colors"
              >
                <X className="w-4 h-4" style={{ color: 'var(--text-muted)' }} />
              </button>
            </div>
            
            {error && (
              <div className="flex items-center text-sm" style={{ gap: '8px', padding: '12px', borderRadius: '12px', marginBottom: '16px', background: 'rgba(239, 68, 68, 0.1)', border: '1px solid rgba(239, 68, 68, 0.2)', color: 'var(--danger)' }}>
                <AlertCircle className="w-4 h-4 flex-shrink-0" />
                {error}
              </div>
            )}

            <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
              {/* Torrent File */}
              <div>
                <label className="text-xs font-semibold uppercase tracking-wide flex items-center" style={{ marginBottom: '8px', gap: '6px', color: 'var(--text-muted)' }}>
                  <File className="w-3.5 h-3.5" /> Torrent File
                </label>
                <div className="flex" style={{ gap: '8px' }}>
                  <input
                    type="text"
                    value={torrentFile}
                    readOnly
                    placeholder="No file selected"
                    className="flex-1 rounded-xl text-sm border border-[var(--border)] focus:border-[var(--accent)] focus:outline-none transition-colors cursor-pointer"
                    style={{ height: '44px', padding: '0 14px', background: 'var(--bg)', color: torrentFile ? 'var(--text)' : 'var(--text-muted)', fontFamily: "'JetBrains Mono', monospace", fontSize: '12px' }}
                    onClick={handleSelectTorrentFile}
                  />
                  <button
                    onClick={handleSelectTorrentFile}
                    className="rounded-xl text-sm font-medium border border-[var(--border)] hover:bg-[var(--border-strong)] transition-colors"
                    style={{ height: '44px', padding: '0 16px', color: 'var(--text-secondary)' }}
                  >
                    Browse
                  </button>
                </div>
              </div>

              {/* Divider */}
              <div className="flex items-center" style={{ gap: '12px', padding: '4px 0' }}>
                <div className="flex-1 h-px bg-[var(--border)]" />
                <span className="text-[10px] font-bold uppercase tracking-widest" style={{ color: 'var(--text-muted)' }}>or</span>
                <div className="flex-1 h-px bg-[var(--border)]" />
              </div>

              {/* Magnet Link */}
              <div>
                <label className="text-xs font-semibold uppercase tracking-wide flex items-center" style={{ marginBottom: '8px', gap: '6px', color: 'var(--text-muted)' }}>
                  <Link2 className="w-3.5 h-3.5" /> Magnet Link
                </label>
                <input
                  type="text"
                  value={magnetInput}
                  onChange={e => { setMagnetInput(e.target.value); if (e.target.value) setTorrentFile(''); }}
                  placeholder="magnet:?xt=urn:btih:..."
                  className="w-full rounded-xl text-sm border border-[var(--border)] focus:border-[var(--accent)] focus:outline-none transition-colors"
                  style={{ height: '44px', padding: '0 14px', background: 'var(--bg)', color: 'var(--text)', fontFamily: "'JetBrains Mono', monospace", fontSize: '12px' }}
                />
              </div>

              {/* Save Location */}
              <div>
                <label className="text-xs font-semibold uppercase tracking-wide flex items-center" style={{ marginBottom: '8px', gap: '6px', color: 'var(--text-muted)' }}>
                  <FolderOpen className="w-3.5 h-3.5" /> Save Location
                </label>
                <div className="flex" style={{ gap: '8px' }}>
                  <input
                    type="text"
                    value={outputPath}
                    onChange={e => setOutputPath(e.target.value)}
                    placeholder="./downloads"
                    className="flex-1 rounded-xl text-sm border border-[var(--border)] focus:border-[var(--accent)] focus:outline-none transition-colors"
                    style={{ height: '44px', padding: '0 14px', background: 'var(--bg)', color: 'var(--text)', fontFamily: "'JetBrains Mono', monospace", fontSize: '12px' }}
                  />
                  <button
                    onClick={handleSelectOutputFolder}
                    className="rounded-xl text-sm font-medium border border-[var(--border)] hover:bg-[var(--border-strong)] transition-colors"
                    style={{ height: '44px', padding: '0 16px', color: 'var(--text-secondary)' }}
                  >
                    Browse
                  </button>
                </div>
              </div>
            </div>

            {/* Footer */}
            <div className="flex" style={{ gap: '12px', marginTop: '24px' }}>
              <button
                onClick={() => setShowAddModal(false)}
                className="flex-1 rounded-xl font-semibold text-sm border border-[var(--border)] transition-all hover:bg-[var(--border-strong)]"
                style={{ height: '44px', color: 'var(--text-secondary)' }}
              >
                Cancel
              </button>
              <button
                onClick={handleAdd}
                disabled={isLoading || (!magnetInput.trim() && !torrentFile)}
                className="flex-1 rounded-xl font-semibold text-sm flex items-center justify-center transition-all duration-200 hover:scale-[1.01] active:scale-[0.99] disabled:opacity-40 disabled:pointer-events-none"
                style={{ 
                  height: '44px',
                  gap: '8px',
                  background: 'linear-gradient(135deg, var(--accent) 0%, #059669 100%)',
                  color: 'white',
                  boxShadow: '0 2px 8px var(--accent-glow)'
                }}
              >
                {isLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Plus className="w-4 h-4" />}
                Add
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function TorrentSection({ title, count, children, accent }: { title: string; count: number; children: React.ReactNode; accent?: boolean }) {
  const [expanded, setExpanded] = useState(true);
  
  return (
    <div className="glass rounded-2xl overflow-hidden border border-[var(--border)] shadow-[var(--shadow-sm)]">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center hover:bg-[var(--border)] transition-colors"
        style={{ height: '48px', padding: '0 20px', gap: '12px' }}
      >
        <ChevronDown 
          className={`w-4 h-4 transition-transform duration-200 ${expanded ? '' : '-rotate-90'}`} 
          style={{ color: 'var(--text-muted)' }} 
        />
        <span 
          className="text-xs font-bold uppercase tracking-wide"
          style={{ color: accent ? 'var(--accent)' : 'var(--text-muted)' }}
        >
          {title}
        </span>
        <span 
          className="text-[10px] font-bold rounded-md"
          style={{ padding: '2px 8px', background: accent ? 'var(--accent-glow)' : 'var(--border)', color: accent ? 'var(--accent)' : 'var(--text-muted)' }}
        >
          {count}
        </span>
      </button>
      {expanded && (
        <div className="border-t border-[var(--border)]">
          {children}
        </div>
      )}
    </div>
  );
}

function TorrentCard({ torrent, selected, onSelect, onRemove, onPause, onResume, delay = 0 }: { 
  torrent: TorrentStatus; 
  selected: boolean; 
  onSelect: () => void; 
  onRemove: () => void;
  onPause: () => void;
  onResume: () => void;
  delay?: number;
}) {
  const remaining = torrent.size - torrent.downloaded;
  const eta = formatETA(remaining, torrent.downSpeed);
  const isActive = torrent.status === 'downloading' || torrent.status === 'starting';
  const isPaused = torrent.status === 'paused';
  const canPauseResume = isActive || isPaused || torrent.status === 'error';

  return (
    <div
      onClick={onSelect}
      className={`group cursor-pointer transition-all duration-200 border-b last:border-b-0 border-[var(--border)] animate-fade-in ${
        selected ? 'bg-[var(--accent-glow)]' : 'hover:bg-[var(--border)]'
      }`}
      style={{ padding: '14px 20px', animationDelay: `${delay}ms` }}
    >
      {/* Top Row */}
      <div className="flex items-center justify-between" style={{ marginBottom: '12px' }}>
        <div className="flex items-center min-w-0 flex-1" style={{ gap: '10px' }}>
          {/* Status Icon */}
          <div className={`w-7 h-7 rounded-lg flex items-center justify-center flex-shrink-0 ${isActive ? 'animate-pulse' : ''}`}
            style={{ 
              background: torrent.status === 'completed' 
                ? 'var(--accent-glow)' 
                : torrent.status === 'error' 
                  ? 'rgba(239, 68, 68, 0.15)' 
                  : 'var(--border)'
            }}
          >
            {torrent.status === 'completed' ? (
              <CheckCircle2 className="w-3.5 h-3.5" style={{ color: 'var(--accent)' }} />
            ) : torrent.status === 'error' ? (
              <AlertCircle className="w-3.5 h-3.5" style={{ color: 'var(--danger)' }} />
            ) : torrent.status === 'starting' ? (
              <Loader2 className="w-3.5 h-3.5 animate-spin" style={{ color: 'var(--accent)' }} />
            ) : torrent.status === 'paused' ? (
              <Pause className="w-3.5 h-3.5" style={{ color: 'var(--text-muted)' }} />
            ) : (
              <Download className="w-3.5 h-3.5" style={{ color: 'var(--accent)' }} />
            )}
          </div>
          
          {/* Name */}
          <span 
            className="font-semibold text-sm truncate"
            style={{ color: 'var(--text)' }}
          >
            {torrent.name || 'Fetching metadata...'}
          </span>
        </div>

        {/* Actions & Progress */}
        <div className="flex items-center flex-shrink-0" style={{ gap: '8px' }}>
          <span 
            className="text-xs font-bold tabular-nums w-10 text-right"
            style={{ color: 'var(--accent)', fontFamily: "'JetBrains Mono', monospace" }}
          >
            {torrent.progress.toFixed(0)}%
          </span>
          {canPauseResume && (
            <button 
              onClick={e => { e.stopPropagation(); isPaused || torrent.status === 'error' ? onResume() : onPause(); }} 
              className="w-7 h-7 rounded-lg flex items-center justify-center hover:bg-[var(--accent-glow)] transition-all opacity-0 group-hover:opacity-100"
              title={isPaused || torrent.status === 'error' ? 'Resume' : 'Pause'}
            >
              {isPaused || torrent.status === 'error' ? (
                <Play className="w-3.5 h-3.5" style={{ color: 'var(--accent)' }} />
              ) : (
                <Pause className="w-3.5 h-3.5" style={{ color: 'var(--text-muted)' }} />
              )}
            </button>
          )}
          <button 
            onClick={e => { e.stopPropagation(); onRemove(); }} 
            className="w-7 h-7 rounded-lg flex items-center justify-center hover:bg-red-500/20 transition-all opacity-0 group-hover:opacity-100"
            title="Remove"
          >
            <Trash2 className="w-3.5 h-3.5" style={{ color: 'var(--danger)' }} />
          </button>
        </div>
      </div>

      {/* Progress Bar */}
      <div className="h-1.5 rounded-full overflow-hidden" style={{ background: 'var(--border)', marginBottom: '12px' }}>
        <div
          className={`h-full rounded-full transition-all duration-500 ${isActive ? 'animate-shimmer' : ''}`}
          style={{ 
            width: `${torrent.progress}%`,
            background: torrent.status === 'error' 
              ? 'var(--danger)' 
              : 'linear-gradient(90deg, var(--accent) 0%, #059669 100%)',
          }}
        />
      </div>

      {/* Stats */}
      <div 
        className="flex items-center text-[11px]"
        style={{ gap: '20px', color: 'var(--text-muted)', fontFamily: "'JetBrains Mono', monospace" }}
      >
        {isActive ? (
          <>
            <span className="flex items-center" style={{ gap: '4px' }}>
              <Download className="w-3 h-3" style={{ color: 'var(--accent)' }} />
              {formatSpeed(torrent.downSpeed)}
            </span>
            <span className="flex items-center" style={{ gap: '4px' }}>
              <Users className="w-3 h-3" />
              {torrent.peers}
            </span>
            <span className="flex items-center" style={{ gap: '4px' }}>
              <Clock className="w-3 h-3" />
              {eta}
            </span>
          </>
        ) : (
          <span className="flex items-center" style={{ gap: '4px' }}>
            <HardDrive className="w-3 h-3" />
            {formatBytes(torrent.size)}
          </span>
        )}
      </div>
    </div>
  );
}

export default App;
