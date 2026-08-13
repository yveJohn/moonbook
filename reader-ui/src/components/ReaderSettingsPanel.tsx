import { useMemo, useState } from 'react';
import type { CSSProperties } from 'react';
import { Card } from 'animal-island-ui';
import type { ReaderFontFamily, ReaderIndentMode, ReaderLocalPreference, ReaderMode } from '../types/reader';

interface ReaderSettingsPanelProps {
  preference: ReaderLocalPreference;
  onChange: (preference: ReaderLocalPreference) => void;
}

type PickerKey = 'font' | 'indent' | 'mode';

const fontOptions: Array<{ label: string; value: ReaderFontFamily }> = [
  { label: '系统字体', value: 'system' },
  { label: '宋体', value: 'serif' },
  { label: '黑体', value: 'hei' },
  { label: '楷体', value: 'kai' }
];

const indentOptions: Array<{ label: string; value: ReaderIndentMode }> = [
  { label: '首行缩进', value: 'indent' },
  { label: '无缩进', value: 'none' }
];

const modeOptions: Array<{ label: string; value: ReaderMode }> = [
  { label: '上下滚动', value: 'scroll' },
  { label: '翻页阅读', value: 'page' }
];

const fontSizeMin = 12;
const fontSizeMax = 28;
const lineHeightSteps = ['1.5', '1.6', '1.7', '1.8', '1.9', '2.0', '2.1'];
const choiceWindowClass = 'reader-settings__choice-window reader-settings__choice-window--full';

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max);
}

function rangePercent(value: number, min: number, max: number) {
  return `${((value - min) / (max - min)) * 100}%`;
}

function fontLabel(fontFamily: ReaderFontFamily) {
  return fontOptions.find((option) => option.value === fontFamily)?.label || fontOptions[0].label;
}

function lineHeightIndex(lineHeight: string) {
  const current = Number(lineHeight);
  if (!Number.isFinite(current)) {
    return 3;
  }
  return lineHeightSteps.reduce((closestIndex, step, index) => {
    const closestDistance = Math.abs(Number(lineHeightSteps[closestIndex]) - current);
    const nextDistance = Math.abs(Number(step) - current);
    return nextDistance < closestDistance ? index : closestIndex;
  }, 3);
}

function rangeStyle(percent: string): CSSProperties {
  return { '--reader-range-percent': percent } as CSSProperties;
}

function CollapseIcon() {
  return (
    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M6 9l6 6 6-6" />
    </svg>
  );
}

export function ReaderSettingsPanel({ preference, onChange }: ReaderSettingsPanelProps) {
  const [picker, setPicker] = useState<PickerKey | null>(null);
  const fontPercent = useMemo(() => rangePercent(preference.fontSize, fontSizeMin, fontSizeMax), [preference.fontSize]);
  const safeMarginSize = clamp(Math.round(preference.marginSize), 0, 6);
  const marginPercent = useMemo(() => rangePercent(safeMarginSize, 0, 6), [safeMarginSize]);
  const safeLineHeightIndex = lineHeightIndex(preference.lineHeight);
  const lineHeightPercent = useMemo(() => rangePercent(safeLineHeightIndex, 0, 6), [safeLineHeightIndex]);

  const update = (patch: Partial<ReaderLocalPreference>) => {
    onChange({ ...preference, ...patch });
  };

  const renderPicker = () => {
    if (!picker) {
      return null;
    }
    if (picker === 'font') {
      return (
        <div
          className={choiceWindowClass}
          role="group"
          aria-label="字体选择"
          onClick={(event) => event.stopPropagation()}
          onPointerDown={(event) => event.stopPropagation()}
        >
          <div className="reader-settings__choice-head">
            <button type="button" className="reader-settings__collapse" aria-label="收起字体选择" onClick={() => setPicker(null)}>
              <CollapseIcon />
            </button>
            <strong className="reader-settings__choice-title">字体选择</strong>
          </div>
          <div className="reader-settings__picker" role="radiogroup" aria-label="字体选择选项">
            {fontOptions.map((option) => (
              <button
                type="button"
                key={option.value}
                className={preference.fontFamily === option.value ? 'is-selected' : ''}
                aria-pressed={preference.fontFamily === option.value}
                onClick={() => {
                  update({ fontFamily: option.value });
                  setPicker(null);
                }}
              >
                {option.label}
              </button>
            ))}
          </div>
        </div>
      );
    }
    if (picker === 'indent') {
      return (
        <div
          className={choiceWindowClass}
          role="group"
          aria-label="首行缩进"
          onClick={(event) => event.stopPropagation()}
          onPointerDown={(event) => event.stopPropagation()}
        >
          <div className="reader-settings__choice-head">
            <button type="button" className="reader-settings__collapse" aria-label="收起首行缩进" onClick={() => setPicker(null)}>
              <CollapseIcon />
            </button>
            <strong className="reader-settings__choice-title">首行缩进</strong>
          </div>
          <div className="reader-settings__picker" role="radiogroup" aria-label="首行缩进选项">
            {indentOptions.map((option) => (
              <button
                type="button"
                key={option.value}
                className={preference.indentMode === option.value ? 'is-selected' : ''}
                aria-pressed={preference.indentMode === option.value}
                onClick={() => {
                  update({ indentMode: option.value });
                  setPicker(null);
                }}
              >
                {option.label}
              </button>
            ))}
          </div>
        </div>
      );
    }
    return (
      <div
        className={choiceWindowClass}
        role="group"
        aria-label="阅读方式"
        onClick={(event) => event.stopPropagation()}
        onPointerDown={(event) => event.stopPropagation()}
      >
        <div className="reader-settings__choice-head">
          <button type="button" className="reader-settings__collapse" aria-label="收起阅读方式" onClick={() => setPicker(null)}>
            <CollapseIcon />
          </button>
          <strong className="reader-settings__choice-title">阅读方式</strong>
        </div>
        <div className="reader-settings__picker" role="radiogroup" aria-label="阅读方式选项">
          {modeOptions.map((option) => (
            <button
              type="button"
              key={option.value}
              className={preference.readingMode === option.value ? 'is-selected' : ''}
              aria-pressed={preference.readingMode === option.value}
              onClick={() => {
                update({ readingMode: option.value });
                setPicker(null);
              }}
            >
              {option.label}
            </button>
          ))}
        </div>
      </div>
    );
  };

  return (
    <Card className="reader-settings">
      <div className="reader-setting-range reader-setting-range--font">
        <div
          className="reader-setting-range__control"
          data-value={preference.fontSize}
          style={rangeStyle(fontPercent)}
        >
          <span className="reader-setting-range__mark reader-setting-range__mark--small">A</span>
          <span className="reader-setting-range__mark reader-setting-range__mark--large">A</span>
          <input
            aria-label="字号"
            max={fontSizeMax}
            min={fontSizeMin}
            type="range"
            value={preference.fontSize}
            onChange={(event) => update({ fontSize: clamp(event.currentTarget.valueAsNumber, fontSizeMin, fontSizeMax) })}
          />
        </div>
      </div>

      <div className="reader-settings__range-grid">
        <div className="reader-setting-range reader-setting-range--compact">
          <div className="reader-setting-range__control" data-value="边距" style={rangeStyle(marginPercent)}>
            <span className="reader-setting-range__mark reader-setting-range__mark--small">小</span>
            <span className="reader-setting-range__mark reader-setting-range__mark--large">大</span>
            <input
              aria-label="边距大小"
              max="6"
              min="0"
              step="1"
              type="range"
              value={safeMarginSize}
              onChange={(event) => update({ marginSize: clamp(event.currentTarget.valueAsNumber, 0, 6) })}
            />
          </div>
        </div>
        <div className="reader-setting-range reader-setting-range--compact">
          <div className="reader-setting-range__control" data-value="行距" style={rangeStyle(lineHeightPercent)}>
            <span className="reader-setting-range__mark reader-setting-range__mark--small">紧</span>
            <span className="reader-setting-range__mark reader-setting-range__mark--large">松</span>
            <input
              aria-label="行距松紧"
              max="6"
              min="0"
              step="1"
              type="range"
              value={safeLineHeightIndex}
              onChange={(event) => update({ lineHeight: lineHeightSteps[clamp(event.currentTarget.valueAsNumber, 0, 6)] })}
            />
          </div>
        </div>
      </div>

      <div className="reader-settings__choices">
        <button
          type="button"
          className={picker === 'font' ? 'reader-setting-choice is-open' : 'reader-setting-choice'}
          onClick={() => setPicker((value) => (value === 'font' ? null : 'font'))}
        >
          <strong>{fontLabel(preference.fontFamily)}</strong>
          <span className="reader-setting-choice__chevron" aria-hidden="true">&gt;</span>
        </button>
        <button
          type="button"
          className={picker === 'indent' ? 'reader-setting-choice is-open' : 'reader-setting-choice'}
          onClick={() => setPicker((value) => (value === 'indent' ? null : 'indent'))}
        >
          <strong>{preference.indentMode === 'indent' ? '首行缩进' : '无缩进'}</strong>
          <span className="reader-setting-choice__chevron" aria-hidden="true">&gt;</span>
        </button>
        <button
          type="button"
          className={picker === 'mode' ? 'reader-setting-choice is-open' : 'reader-setting-choice'}
          onClick={() => setPicker((value) => (value === 'mode' ? null : 'mode'))}
        >
          <strong>{preference.readingMode === 'page' ? '翻页阅读' : '上下滚动'}</strong>
          <span className="reader-setting-choice__chevron" aria-hidden="true">&gt;</span>
        </button>
      </div>

      {renderPicker()}
    </Card>
  );
}
