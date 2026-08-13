import { Card } from 'animal-island-ui';
import type { ReaderLocalPreference, ReaderTheme } from '../types/reader';

interface ReaderPalettePanelProps {
  preference: ReaderLocalPreference;
  onChange: (preference: ReaderLocalPreference) => void;
}

const themeOptions: Array<{ label: string; value: ReaderTheme }> = [
  { label: '默认', value: 'cream' },
  { label: '护眼', value: 'green' },
  { label: '夜间', value: 'night' }
];

export function ReaderPalettePanel({ preference, onChange }: ReaderPalettePanelProps) {
  const updateTheme = (theme: ReaderTheme) => {
    onChange({ ...preference, theme });
  };

  return (
    <Card className="reader-palette">
      <div className="reader-palette__options" role="group" aria-label="配色选择">
        {themeOptions.map((option) => (
          <button
            type="button"
            key={option.value}
            className={preference.theme === option.value ? 'reader-palette__option is-selected' : 'reader-palette__option'}
            aria-pressed={preference.theme === option.value}
            onClick={() => updateTheme(option.value)}
          >
            <span className={`reader-palette__swatch reader-palette__swatch--${option.value}`} aria-hidden="true" />
            <span>{option.label}</span>
          </button>
        ))}
      </div>
    </Card>
  );
}
