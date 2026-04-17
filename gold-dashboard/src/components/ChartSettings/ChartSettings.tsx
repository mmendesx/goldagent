import { useState, useRef, useEffect, useId } from 'react';
import { useDashboardStore, selectChartIndicators } from '../../store';
import { Button } from '../../design-system/Button';
import './ChartSettings.css';

interface ChartSettingsProps {
  settingsKey: string; // e.g. 'binance|BTCUSDT|5m'
}

export function ChartSettings({ settingsKey }: ChartSettingsProps) {
  const [isOpen, setIsOpen] = useState(false);
  const panelRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const uid = useId();
  const panelId = `chart-settings-panel-${uid.replace(/:/g, '')}`;

  const settings = useDashboardStore(selectChartIndicators(settingsKey));
  const setChartIndicators = useDashboardStore((s) => s.setChartIndicators);

  // Close on outside click
  useEffect(() => {
    if (!isOpen) return;
    function handleMousedown(e: MouseEvent) {
      if (
        panelRef.current &&
        !panelRef.current.contains(e.target as Node) &&
        triggerRef.current &&
        !triggerRef.current.contains(e.target as Node)
      ) {
        setIsOpen(false);
      }
    }
    document.addEventListener('mousedown', handleMousedown);
    return () => document.removeEventListener('mousedown', handleMousedown);
  }, [isOpen]);

  // Focus trap + ESC dismiss
  useEffect(() => {
    if (!isOpen || !panelRef.current) return;
    const focusable = panelRef.current.querySelectorAll<HTMLElement>(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
    );
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    first?.focus();

    function handler(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        setIsOpen(false);
        triggerRef.current?.focus();
      }
      if (e.key === 'Tab') {
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last?.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first?.focus();
        }
      }
    }
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [isOpen]);

  function updatePeriod(index: 0 | 1, value: string) {
    const n = parseInt(value, 10);
    if (Number.isNaN(n) || n < 2 || n > 200) return;
    const periods = [...settings.ma.periods] as [number, number];
    periods[index] = n;
    setChartIndicators(settingsKey, { ma: { ...settings.ma, periods } });
  }

  return (
    <div className="chart-settings">
      <Button
        ref={triggerRef}
        variant="ghost"
        size="sm"
        className={`chart-settings__trigger${isOpen ? ' chart-settings__trigger--active' : ''}`}
        aria-label="Chart settings"
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        aria-controls={panelId}
        onClick={() => setIsOpen((v) => !v)}
      >
        ⚙
      </Button>

      {isOpen && (
        <div
          ref={panelRef}
          id={panelId}
          className="chart-settings__panel"
          role="dialog"
          aria-label="Chart Settings"
          aria-modal="true"
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
