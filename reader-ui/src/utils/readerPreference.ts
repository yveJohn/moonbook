import type {
  ReaderFontFamily,
  ReaderIndentMode,
  ReaderLocalPreference,
  ReaderMode,
  ReaderPreference,
  ReaderPreferencePayload,
  ReaderTheme
} from '../types/reader';

export const READER_PREFERENCE_KEY = 'moonbook.reader.preference';

export const fallbackReaderPreference: ReaderLocalPreference = {
  fontSize: 20,
  lineHeight: '1.8',
  theme: 'cream',
  readingMode: 'page',
  marginSize: 3,
  indentMode: 'indent',
  fontFamily: 'system'
};

const readerThemes: ReaderTheme[] = ['cream', 'night', 'green'];
const readerModes: ReaderMode[] = ['scroll', 'page'];
const readerIndentModes: ReaderIndentMode[] = ['indent', 'none'];
const readerFontFamilies: ReaderFontFamily[] = ['system', 'serif', 'hei', 'kai'];

function safeTheme(theme: ReaderPreference['theme']): ReaderTheme {
  return readerThemes.includes(theme) ? theme : fallbackReaderPreference.theme;
}

function safeMode(mode: ReaderPreference['readingMode']): ReaderMode {
  return readerModes.includes(mode) ? mode : fallbackReaderPreference.readingMode;
}

function safeIndentMode(mode: ReaderLocalPreference['indentMode']): ReaderIndentMode {
  return readerIndentModes.includes(mode) ? mode : fallbackReaderPreference.indentMode;
}

function safeFontFamily(fontFamily: ReaderLocalPreference['fontFamily']): ReaderFontFamily {
  return readerFontFamilies.includes(fontFamily) ? fontFamily : fallbackReaderPreference.fontFamily;
}

function safeMarginSize(marginSize: unknown): number {
  if (typeof marginSize !== 'number' || !Number.isFinite(marginSize)) {
    return fallbackReaderPreference.marginSize;
  }
  return Math.min(Math.max(Math.round(marginSize), 0), 6);
}

export function toPreferencePayload(preference: ReaderPreference | ReaderPreferencePayload): ReaderPreferencePayload {
  return {
    fontSize: preference.fontSize || fallbackReaderPreference.fontSize,
    lineHeight: String(preference.lineHeight || fallbackReaderPreference.lineHeight),
    theme: safeTheme(preference.theme),
    readingMode: safeMode(preference.readingMode)
  };
}

export function toLocalReaderPreference(
  preference: ReaderPreference | ReaderPreferencePayload | ReaderLocalPreference
): ReaderLocalPreference {
  const payload = toPreferencePayload(preference);
  const localPreference = preference as Partial<ReaderLocalPreference>;
  return {
    ...payload,
    marginSize: safeMarginSize(localPreference.marginSize),
    indentMode: safeIndentMode(localPreference.indentMode || fallbackReaderPreference.indentMode),
    fontFamily: safeFontFamily(localPreference.fontFamily || fallbackReaderPreference.fontFamily)
  };
}

export function loadLocalReaderPreference(): ReaderLocalPreference {
  if (typeof window === 'undefined') {
    return fallbackReaderPreference;
  }
  const rawPreference = window.localStorage.getItem(READER_PREFERENCE_KEY);
  if (!rawPreference) {
    return fallbackReaderPreference;
  }
  try {
    return toLocalReaderPreference(JSON.parse(rawPreference) as ReaderLocalPreference);
  } catch {
    return fallbackReaderPreference;
  }
}

export function saveLocalReaderPreference(preference: ReaderLocalPreference) {
  if (typeof window === 'undefined') {
    return;
  }
  window.localStorage.setItem(READER_PREFERENCE_KEY, JSON.stringify(toLocalReaderPreference(preference)));
}
