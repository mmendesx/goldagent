import { useState, useRef, useEffect } from 'react';
import { useDashboardStore, selectChartIndicators } from '../../store';
import './ChartSettings.css';

interface ChartSettingsProps {
  settingsKey: string; // e.g. 'binance|BTCUSDT|5m'
}

export function ChartSettings({ settingsKey }: ChartSettingsProps) {
  const [open, setOpen] = useState(false);
  const panelRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);

  const settings = useDashboardStore(selectChartIndicators(settingsKey));
  const setChartIndicators = useDashboardStore((s) => s.setChartIndicators);

  // Close on outside click
  useEffect(() => {
    if (!open) return;
    function handleClick(e: MouseEvent) {
      if (panelRef.current && !panelRef.current.contains(e.target as Node) &&
          buttonRef.current && !buttonRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [open]);

  // Close on Escape
  useEffect(() => {
    if (!open) return;
    function handleKey(e: KeyboardEvent) {
      if (e.key === 'Escape') { setOpen(false); buttonRef.current?.focus(); }
    }
    document.addEventListener('keydown', handleKey);
    return () => document.removeEventListener('keydown', handleKey);
  }, [open]);

  function updatePeriod(index: 0 | 1, value: string) {
    const n = parseInt(value, 10);
    if (Number.isNaN(n) || n < 2 || n > 200) return;
    const periods = [...settings.ma.periods] as [number, number];
    periods[index] = n;
    setChartIndicators(settingsKey, { ma: { ...settings.ma, periods } });
  }

  return (
    <div className="chart-settings">
      <button
        ref={buttonRef}
        type="button"
        className={`chart-settings__trigger${open ? ' chart-settings__trigger--active' : ''}`}
        aria-label="Chart settings"
        aria-expanded={open}
        aria-haspopup="dialog"
        onClick={() => setOpen((v) => !v)}
      >
        ⚙
      </button>

      {open && (
        <div
          ref={panelRef}
          className="chart-settings__panel"
          role="dialog"
          aria-label="Chart indicator settings"
        >
          {/* MA toggle */}
          <div className="chart-settings__row">
            <label className="chart-settings__toggle">
              <input
                type="checkbox"
                checked={settings.ma.enabled}
                onChange={(e) =>
                  setChartIndicators(settingsKey, { ma: { ...settings.ma, enabled: e.target.checked } })
                }
                aria-label="Enable moving averages"
              />
              <span>Moving Averages</span>
            </label>
          </div>
          {settings.ma.enabled && (
            <div className="chart-settings__periods">
              <label className="chart-settings__period-label">
                MA 1
                <input
                  type="number"
                  min={2} max={200}
                  value={settings.ma.periods[0]}
                  onChange={(e) => updatePeriod(0, e.target.value)}
                  className="chart-settings__period-input"
                  aria-label="MA period 1"
                />
              </label>
              <label className="chart-settings__period-label">
                MA 2
                <input
                  type="number"
                  min={2} max={200}
                  value={settings.ma.periods[1]}
                  onChange={(e) => updatePeriod(1, e.target.value)}
                  className="chart-settings__period-input"
                  aria-label="MA period 2"
                />
              </label>
            </div>
          )}

          {/* VWAP toggle */}
          <div className="chart-settings__row">
            <label className="chart-settings__toggle">
              <input
                type="checkbox"
                checked={settings.vwap.enabled}
                onChange={(e) =>
                  setChartIndicators(settingsKey, { vwap: { enabled: e.target.checked } })
                }
                aria-label="Enable VWAP"
              />
              <span>VWAP</span>
            </label>
          </div>

          {/* Volume toggle */}
          <div className="chart-settings__row">
            <label className="chart-settings__toggle">
              <input
                type="checkbox"
                checked={settings.volume.enabled}
                onChange={(e) =>
                  setChartIndicators(settingsKey, { volume: { enabled: e.target.checked } })
                }
                aria-label="Enable volume pane"
              />
              <span>Volume</span>
            </label>
          </div>
        </div>
      )}
    </div>
  );
}
