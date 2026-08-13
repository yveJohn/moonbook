import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';

const globalCss = readFileSync('src/styles/global.css', 'utf8');

function cssBlock(selector: string) {
  const start = globalCss.indexOf(`${selector} {`);
  expect(start).toBeGreaterThanOrEqual(0);
  const blockStart = globalCss.indexOf('{', start);
  const blockEnd = globalCss.indexOf('}', blockStart);
  return globalCss.slice(blockStart + 1, blockEnd);
}

function cssProperty(block: string, name: string) {
  const match = block.match(new RegExp(`${name}:\\s*([^;]+);`));
  expect(match).toBeTruthy();
  return match?.[1].trim();
}

function relativeLuminance(hex: string) {
  const match = /^#([\da-f]{2})([\da-f]{2})([\da-f]{2})$/i.exec(hex);
  expect(match).toBeTruthy();
  const channels = match!.slice(1).map((channel) => Number.parseInt(channel, 16) / 255)
    .map((channel) => channel <= 0.04045
      ? channel / 12.92
      : ((channel + 0.055) / 1.055) ** 2.4);
  return 0.2126 * channels[0] + 0.7152 * channels[1] + 0.0722 * channels[2];
}

function contrastRatio(foreground: string, background: string) {
  const foregroundLuminance = relativeLuminance(foreground);
  const backgroundLuminance = relativeLuminance(background);
  return (Math.max(foregroundLuminance, backgroundLuminance) + 0.05)
    / (Math.min(foregroundLuminance, backgroundLuminance) + 0.05);
}

function cssBlockForSelector(selector: string) {
  const blocks = Array.from(globalCss.matchAll(/([^{}]+)\{([^{}]*)\}/g));
  const match = blocks.find(([, selectors]) =>
    selectors
      .split(',')
      .map((item) => item.trim())
      .includes(selector),
  );

  expect(match).toBeTruthy();
  return match?.[2] ?? '';
}

function closingBrace(source: string, openingBrace: number) {
  let depth = 0;
  for (let index = openingBrace; index < source.length; index += 1) {
    if (source[index] === '{') depth += 1;
    if (source[index] === '}') depth -= 1;
    if (depth === 0) return index;
  }
  throw new Error(`Unclosed CSS block at ${openingBrace}`);
}

function cssMediaBlock(query: string) {
  const mediaStart = globalCss.indexOf(`@media ${query} {`);
  expect(mediaStart).toBeGreaterThanOrEqual(0);
  const mediaOpening = globalCss.indexOf('{', mediaStart);
  return globalCss.slice(mediaOpening + 1, closingBrace(globalCss, mediaOpening));
}

function cssMediaBlocks(query: string) {
  const marker = `@media ${query} {`;
  const blocks: string[] = [];
  let searchFrom = 0;

  while (searchFrom < globalCss.length) {
    const mediaStart = globalCss.indexOf(marker, searchFrom);
    if (mediaStart < 0) break;
    const mediaOpening = globalCss.indexOf('{', mediaStart);
    const mediaClosing = closingBrace(globalCss, mediaOpening);
    blocks.push(globalCss.slice(mediaOpening + 1, mediaClosing));
    searchFrom = mediaClosing + 1;
  }

  expect(blocks.length).toBeGreaterThan(0);
  return blocks;
}

function cssBlockWithinMedia(query: string, selector: string) {
  const mediaBody = cssMediaBlock(query);
  const selectorStart = mediaBody.indexOf(`${selector} {`);
  expect(selectorStart).toBeGreaterThanOrEqual(0);
  const selectorOpening = mediaBody.indexOf('{', selectorStart);
  return mediaBody.slice(selectorOpening + 1, closingBrace(mediaBody, selectorOpening));
}

describe('reader setting range styles', () => {
  it('lets the slider thumb cover edge labels while staying draggable', () => {
    const thumbBlock = cssBlock('.reader-setting-range__control::after');
    const markBlock = cssBlock('.reader-setting-range__mark');
    const inputBlock = cssBlock(".reader-setting-range input[type='range']");

    expect(cssProperty(thumbBlock, 'left')).toBe('clamp(15px, var(--reader-range-percent), calc(100% - 15px))');
    expect(Number(cssProperty(thumbBlock, 'z-index'))).toBeGreaterThan(Number(cssProperty(markBlock, 'z-index')));
    expect(Number(cssProperty(inputBlock, 'z-index'))).toBeGreaterThan(Number(cssProperty(thumbBlock, 'z-index')));
  });
});

describe('reader list layout styles', () => {
  it('uses the approved sub-category panel palette and wrapping layout', () => {
    const panelBlock = cssBlock('.sub-category-panel');
    const optionsBlock = cssBlock('.sub-category-panel__options');
    const chipBlock = cssBlock('.sub-category-chip');
    const collapseBlock = cssBlock('.sub-category-panel__collapse');

    expect(cssProperty(panelBlock, 'background')).toBe('#fefaf4');
    expect(cssProperty(optionsBlock, 'flex-wrap')).toBe('wrap');
    expect(cssProperty(chipBlock, 'background')).toBe('#fcf6ec');
    expect(cssProperty(chipBlock, 'color')).toBe('#ab9b89');
    expect(cssProperty(collapseBlock, 'color')).toBe('#ab9b89');
  });

  it('keeps single-column grid tracks shrinkable to prevent page overflow', () => {
    expect(cssProperty(cssBlock('.reader-page'), 'grid-template-columns')).toBe('minmax(0, 1fr)');
    expect(cssProperty(cssBlock('.book-list'), 'grid-template-columns')).toBe('minmax(0, 1fr)');
    expect(cssProperty(cssBlock('.book-row__body'), 'grid-template-columns')).toBe('minmax(0, 1fr)');
  });

  it('keeps bookshelf rows shrinkable and edit actions stable', () => {
    expect(cssProperty(cssBlock('.bookshelf-list'), 'grid-template-columns')).toBe('minmax(0, 1fr)');
    expect(cssProperty(cssBlock('.bookshelf-row'), 'grid-template-columns')).toBe('minmax(0, 1fr) auto');
    expect(cssProperty(cssBlock('.bookshelf-row--editing'), 'grid-template-columns')).toBe('auto minmax(0, 1fr)');
    expect(cssProperty(cssBlock('.bookshelf-editbar'), 'grid-template-columns')).toBe('1fr 1.4fr');
    expect(cssProperty(cssBlock('.bookshelf-row__title'), 'text-overflow')).toBe('ellipsis');
    expect(cssProperty(cssBlockForSelector('.bookshelf-row__meta'), 'text-overflow')).toBe('ellipsis');
    expect(cssProperty(cssBlockForSelector('.bookshelf-row__time'), 'text-overflow')).toBe('ellipsis');
    expect(cssProperty(cssBlockForSelector('.bookshelf-row__progress-text'), 'text-overflow')).toBe('ellipsis');
    expect(cssProperty(cssBlock('.bookshelf-progress'), 'overflow')).toBe('hidden');
  });

  it('uses the liked book row surface radius', () => {
    expect(cssProperty(cssBlock('.liked-book-row'), 'border-radius')).toBe('16px');
    expect(cssProperty(cssBlock('.liked-book-row__meta'), 'color')).toBe('#68736d');
  });

  it('keeps liked book headings shrinkable on mobile', () => {
    expect(cssProperty(cssBlockWithinMedia('(max-width: 480px)', '.liked-book-row__heading'), 'width')).toBe('100%');
    expect(cssProperty(cssBlockWithinMedia('(max-width: 480px)', '.liked-book-row__title'), 'max-width')).toBe('100%');
    expect(cssProperty(cssBlockWithinMedia('(max-width: 480px)', '.liked-book-row__author'), 'max-width')).toBe('100%');
  });

  it('shows disabled app bar actions as unavailable', () => {
    const disabledLinkBlock = cssBlock('.app-bar__link:disabled');

    expect(cssProperty(disabledLinkBlock, 'opacity')).toBe('0.55');
    expect(cssProperty(disabledLinkBlock, 'cursor')).toBe('not-allowed');
  });

  it('locks the books page while letting the book list own vertical scrolling', () => {
    const shellBodyBlock = cssBlock('.app-shell:has(.books-page) .app-shell__body');
    const pageBlock = cssBlock('.books-page');
    const resultsBlock = cssBlock('.books-results');
    const listBlock = cssBlock('.books-page .book-list');

    expect(cssProperty(shellBodyBlock, 'height')).toBe('100dvh');
    expect(cssProperty(shellBodyBlock, 'overflow')).toBe('hidden');
    expect(cssProperty(pageBlock, 'height')).toBe('100%');
    expect(cssProperty(pageBlock, 'grid-template-rows')).toBe('auto minmax(0, 1fr)');
    expect(cssProperty(pageBlock, 'overflow')).toBe('hidden');
    expect(cssProperty(resultsBlock, 'grid-template-rows')).toBe('auto minmax(0, 1fr)');
    expect(cssProperty(resultsBlock, 'min-height')).toBe('0');
    expect(cssProperty(listBlock, 'grid-row')).toBe('2');
    expect(cssProperty(listBlock, 'min-height')).toBe('0');
    expect(cssProperty(listBlock, 'overflow-y')).toBe('auto');
    expect(cssProperty(listBlock, 'overscroll-behavior')).toBe('contain');
  });

  it('keeps sparse books page rows at their content height', () => {
    const listBlock = cssBlock('.books-page .book-list');

    expect(cssProperty(listBlock, 'align-content')).toBe('start');
    expect(cssProperty(listBlock, 'align-items')).toBe('start');
    expect(cssProperty(listBlock, 'grid-auto-rows')).toBe('max-content');
  });

  it('keeps books page row titles and descriptions compact', () => {
    const baseTitleBlock = cssBlock('.book-row__title');
    const titleBlock = cssBlock('.books-page .book-row__title');
    const descriptionBlock = cssBlock('.books-page .book-row__description');

    expect(cssProperty(baseTitleBlock, 'overflow')).toBe('hidden');
    expect(cssProperty(baseTitleBlock, 'text-overflow')).toBe('ellipsis');
    expect(cssProperty(baseTitleBlock, 'white-space')).toBe('nowrap');
    expect(cssProperty(titleBlock, 'display')).toBe('-webkit-box');
    expect(cssProperty(titleBlock, 'font-size')).toBe('0.93rem');
    expect(cssProperty(titleBlock, 'letter-spacing')).toBe('0.08rem');
    expect(cssProperty(titleBlock, '-webkit-line-clamp')).toBe('1');
    expect(cssProperty(descriptionBlock, 'color')).toBe('#898480');
    expect(cssProperty(descriptionBlock, 'font-size')).toBe('0.8rem');
    expect(cssProperty(descriptionBlock, 'letter-spacing')).toBe('0.08rem');
  });

  it('styles the books page sort control as a muted rounded rectangle', () => {
    const statusBlock = cssBlock('.books-status');
    const sortButtonBlock = cssBlock('.books-sort__toggle');

    expect(cssProperty(statusBlock, 'display')).toBe('flex');
    expect(cssProperty(statusBlock, 'justify-content')).toBe('space-between');
    expect(cssProperty(sortButtonBlock, 'border-radius')).toBe('999px');
    expect(cssProperty(sortButtonBlock, 'background')).toBe('#faf4ea');
    expect(cssProperty(sortButtonBlock, 'color')).toBe('#8f8679');
  });

  it('uses compact spaced typography on the book detail title and description', () => {
    const titleBlock = cssBlock('.detail-head__title');
    const descriptionBlock = cssBlock('.detail-description__text');

    expect(cssProperty(titleBlock, 'display')).toBe('-webkit-box');
    expect(cssProperty(titleBlock, 'font-size')).toBe('clamp(1.43rem, 6.2vw, 1.98rem)');
    expect(cssProperty(titleBlock, 'letter-spacing')).toBe('0.08rem');
    expect(cssProperty(titleBlock, '-webkit-line-clamp')).toBe('2');
    expect(cssProperty(descriptionBlock, 'color')).toBe('#898480');
    expect(cssProperty(descriptionBlock, 'font-size')).toBe('0.8rem');
    expect(cssProperty(descriptionBlock, 'letter-spacing')).toBe('0.08rem');
  });
});

describe('reader account control accessibility styles', () => {
  it('keeps the app background solid and bottom navigation labels AA compliant', () => {
    const body = cssBlock('body');
    const background = '#f8f3e8';
    const normal = cssProperty(cssBlock('.tab-bar__item'), 'color');
    const active = cssProperty(cssBlock('.tab-bar__item.is-active'), 'color');

    expect(cssProperty(body, 'background')).toBe(background);
    expect(body).not.toContain('linear-gradient');
    expect(normal).toBe('#725d42');
    expect(active).toBe('#2f7564');
    expect(contrastRatio(normal, background)).toBeGreaterThanOrEqual(4.5);
    expect(contrastRatio(active, background)).toBeGreaterThanOrEqual(4.5);
  });

  it('shows keyboard focus and a non-color active marker without shifting tab layout', () => {
    const focus = cssBlock('.tab-bar__item:focus-visible');
    const label = cssBlock('.tab-bar__label');
    const marker = cssBlock('.tab-bar__item.is-active .tab-bar__label::after');

    expect(cssProperty(focus, 'outline')).toBe('2px solid #2f7564');
    expect(cssProperty(focus, 'outline-offset')).toBe('-2px');
    expect(cssProperty(label, 'position')).toBe('relative');
    expect(cssProperty(marker, 'content')).toBe("''");
    expect(cssProperty(marker, 'position')).toBe('absolute');
    expect(cssProperty(marker, 'right')).toBe('0');
    expect(cssProperty(marker, 'left')).toBe('0');
    expect(cssProperty(marker, 'height')).toBe('2px');
    expect(cssProperty(marker, 'bottom')).toBe('-4px');
    expect(cssProperty(marker, 'background')).toBe('currentColor');
  });

  it('keeps navigation and password controls at least 44 by 44 pixels', () => {
    expect(cssProperty(cssBlock('.app-bar__back'), 'width')).toBe('44px');
    expect(cssProperty(cssBlock('.app-bar__back'), 'height')).toBe('44px');
    expect(cssProperty(cssBlock('.app-shell:has(.book-detail-page) .app-bar__back'), 'width')).toBe('44px');
    expect(cssProperty(cssBlock('.app-shell:has(.book-detail-page) .app-bar__back'), 'height')).toBe('44px');
    expect(cssProperty(cssBlock('.password-toggle'), 'width')).toBe('44px');
    expect(cssProperty(cssBlock('.password-toggle'), 'height')).toBe('44px');
    expect(cssProperty(cssBlockForSelector('.settings-submit'), 'min-height')).toBe('44px');
    expect(cssProperty(cssBlockForSelector('.settings-logout-button'), 'min-height')).toBe('44px');
  });

  it('uses solid high-contrast focus outlines on account controls', () => {
    expect(cssProperty(cssBlockForSelector('.app-bar__back:focus-visible'), 'outline')).toBe('2px solid #2f7564');
    expect(cssProperty(cssBlockForSelector('.password-toggle:focus-visible'), 'outline')).toBe('2px solid #2f7564');
    expect(cssProperty(cssBlockForSelector('.settings-submit:focus-visible'), 'outline')).toBe('3px solid #2f7564');
    expect(cssProperty(cssBlockForSelector('.settings-logout-button:focus-visible'), 'outline')).toBe('3px solid #2f7564');
    expect(cssProperty(cssBlockForSelector('.app-bar__back:focus-visible'), 'outline-offset')).toBe('2px');
    expect(cssProperty(cssBlockForSelector('.password-toggle:focus-visible'), 'outline-offset')).toBe('2px');
  });
});

describe('reader chapter drawer styles', () => {
  it('places the chapter drawer as a bottom sheet above the reader dock', () => {
    const drawerBlock = cssBlock('.reader-drawer');
    const panelBlock = cssBlock('.reader-drawer__panel');

    expect(cssProperty(drawerBlock, 'top')).toBe('0');
    expect(cssProperty(drawerBlock, 'bottom')).toBe('var(--reader-dock-height)');
    expect(cssProperty(drawerBlock, 'left')).toBe('0');
    expect(cssProperty(drawerBlock, 'right')).toBe('0');
    expect(cssProperty(panelBlock, 'inset')).toBe('0');
    expect(panelBlock).not.toContain('width: min(88vw, 420px);');
    expect(panelBlock).not.toContain('border-left');
  });
});

describe('reader docked popover styles', () => {
  it('keeps scroll reading content below the fixed reader bar', () => {
    const readerViewBlock = cssBlock('.reader-view');

    expect(cssProperty(readerViewBlock, 'padding')).toBe('calc(76px + env(safe-area-inset-top)) 16px 116px');
    expect(globalCss).toContain('padding: calc(96px + env(safe-area-inset-top)) 96px 112px;');
  });

  it('uses the dock height as the single shared boundary for docked panels', () => {
    const themeBlock = cssBlock('.reader-theme');
    const toolbarBlock = cssBlock('.reader-view__toolbar');
    const settingsDockBlock = cssBlock('.reader-settings-popover--dock');
    const paletteDockBlock = cssBlock('.reader-palette-popover--dock');

    expect(cssProperty(themeBlock, '--reader-dock-height')).toBe('calc(61px + env(safe-area-inset-bottom))');
    expect(cssProperty(toolbarBlock, 'height')).toBe('var(--reader-dock-height)');
    expect(toolbarBlock).not.toContain('min-height: var(--reader-dock-height);');
    expect(cssProperty(settingsDockBlock, 'bottom')).toBe('var(--reader-dock-height)');
    expect(cssProperty(paletteDockBlock, 'bottom')).toBe('var(--reader-dock-height)');
  });

  it('lets paged paragraphs fragment across columns instead of reserving one paragraph per page', () => {
    const paragraphBlock = cssBlock('.reader-pager__track .reader-paragraph');

    expect(paragraphBlock).not.toContain('break-inside: avoid;');
    expect(paragraphBlock).not.toContain('page-break-inside: avoid;');
  });

  it('centers settings choice labels and keeps their overlay above range controls', () => {
    const choiceBlock = cssBlock('.reader-setting-choice');
    const choiceStrongBlock = cssBlock('.reader-setting-choice strong');
    const choiceChevronBlock = cssBlock('.reader-setting-choice__chevron');
    const choiceWindowBlock = cssBlock('.reader-settings__choice-window');
    const rangeInputBlock = cssBlock(".reader-setting-range input[type='range']");

    expect(cssProperty(choiceBlock, 'grid-template-columns')).toBe('minmax(0, 1fr) auto');
    expect(cssProperty(choiceBlock, 'padding')).toBe('6px 8px');
    expect(cssProperty(choiceBlock, 'text-align')).toBe('center');
    expect(cssProperty(choiceStrongBlock, 'text-align')).toBe('center');
    expect(choiceChevronBlock).not.toContain('position: absolute;');
    expect(Number(cssProperty(choiceWindowBlock, 'z-index'))).toBeGreaterThan(Number(cssProperty(rangeInputBlock, 'z-index')));
    expect(cssProperty(choiceWindowBlock, 'pointer-events')).toBe('auto');
  });
});

describe('reader palette swatch styles', () => {
  it('keeps theme swatches crisp without blurred outer shadow', () => {
    const swatchBlock = cssBlock('.reader-palette__swatch');
    const swatchSplitBlock = cssBlock('.reader-palette__swatch::before');
    const swatchInnerBlock = cssBlock('.reader-palette__swatch::after');

    expect(cssProperty(swatchBlock, 'box-shadow')).toBe('none');
    expect(cssProperty(swatchBlock, 'overflow')).toBe('hidden');
    expect(cssProperty(swatchSplitBlock, 'top')).toBe('0');
    expect(cssProperty(swatchSplitBlock, 'right')).toBe('0');
    expect(cssProperty(swatchSplitBlock, 'bottom')).toBe('0');
    expect(cssProperty(swatchSplitBlock, 'width')).toBe('50%');
    expect(swatchSplitBlock).not.toContain('height: 46%;');
    expect(cssProperty(swatchInnerBlock, 'border-radius')).toBe('inherit');
    expect(cssProperty(swatchInnerBlock, 'box-shadow')).toBe('inset 0 0 0 1px rgba(255, 255, 255, 0.42)');
  });
});

describe('reader me page styles', () => {
  it('uses a consistent 16 pixel radius for every top-level me surface', () => {
    const surfaces = [
      '.me-checkin-card',
      '.me-invite-card',
      '.membership-permanent',
      '.membership-plan-card',
      '.me-quick-link',
      '.invite-reward-dialog'
    ];

    surfaces.forEach((selector) => {
      expect(cssProperty(cssBlockForSelector(selector), 'border-radius')).toBe('16px');
    });
  });

  it('uses a horizontal 78 percent membership rail and 3 by 2 media on mobile', () => {
    const plans = cssBlock('.membership-plans');
    const media = cssBlock('.membership-plan-card__media');
    const card = cssBlock('.membership-plan-card');
    const price = cssBlock('.membership-plan-card__price');
    expect(cssProperty(plans, 'grid-auto-flow')).toBe('column');
    expect(cssProperty(plans, 'grid-auto-columns')).toBe('78%');
    expect(cssProperty(plans, 'overflow-x')).toBe('auto');
    expect(cssProperty(plans, 'scroll-snap-type')).toBe('x mandatory');
    expect(cssProperty(plans, 'padding')).toBe('6px 6px 8px');
    expect(cssProperty(media, 'aspect-ratio')).toBe('3 / 2');
    expect(cssProperty(card, 'aspect-ratio')).toBe('3 / 2');
    expect(cssProperty(card, 'border-radius')).toBe('16px');
    expect(cssProperty(price, 'left')).toBe('20%');
    expect(cssProperty(price, 'top')).toBe('69%');
  });

  it('keeps the page single-column and uses exact membership and quick-link grid tracks', () => {
    expect(globalCss).not.toContain('.me-action-grid');
    expect(globalCss).not.toContain('.me-action-primary');
    expect(cssProperty(cssBlock('.me-quick-links'), 'grid-template-columns'))
      .toBe('repeat(4, minmax(0, 1fr))');
    expect(cssProperty(cssBlockWithinMedia('(min-width: 680px)', '.membership-plans'), 'grid-template-columns'))
      .toBe('repeat(2, minmax(0, 1fr))');
    expect(cssProperty(cssBlockWithinMedia('(min-width: 1040px)', '.membership-plans'), 'grid-template-columns'))
      .toBe('repeat(3, minmax(0, 1fr))');
  });

  it('uses a bottom sheet on mobile and a centered dialog on tablet', () => {
    expect(cssProperty(cssBlock('.invite-reward-overlay'), 'align-items')).toBe('end');
    expect(cssProperty(cssBlock('.invite-reward-dialog'), 'border-radius')).toBe('16px');
    expect(cssProperty(cssBlock('.invite-reward-dialog'), 'padding')).toContain('env(safe-area-inset-bottom)');
    expect(cssProperty(cssBlockWithinMedia('(min-width: 680px)', '.invite-reward-overlay'), 'place-items'))
      .toBe('center');
    expect(cssProperty(cssBlockWithinMedia('(min-width: 680px)', '.invite-reward-dialog'), 'border-radius'))
      .toBe('16px');
  });

  it('uses a two-column reward dashboard with stable coin icons', () => {
    expect(cssProperty(cssBlock('.me-invite-reward-grid'), 'grid-template-columns'))
      .toBe('repeat(2, minmax(0, 1fr))');
    expect(cssProperty(cssBlock('.me-invite-reward__coin'), 'width')).toBe('28px');
    expect(cssProperty(cssBlock('.me-invite-reward__coin'), 'height')).toBe('28px');
    expect(cssProperty(cssBlock('.me-invite-stats'), 'grid-template-columns'))
      .toBe('repeat(2, minmax(0, 1fr))');
  });

  it('reserves stable skeleton sizes without the removed wallet section', () => {
    expect(cssProperty(cssBlock('.me-checkin-skeleton'), 'min-height')).toBe('156px');
    expect(cssProperty(cssBlock('.me-invite-skeleton'), 'min-height')).toBe('370px');
    expect(cssProperty(cssBlock('.membership-plans-skeleton'), 'min-height')).toBe('150px');
    expect(globalCss).not.toContain('.me-wallet-section');
    expect(globalCss).not.toContain('.me-wallet-strip');
    expect(globalCss).not.toContain('.me-wallet-skeleton');
  });

  it('keeps profile identity and the wallet links in a permanent three-column header', () => {
    expect(cssProperty(cssBlock('.me-profile-header'), 'grid-template-columns')).toBe('48px minmax(0, 1fr) 112px');
    expect(cssProperty(cssBlock('.me-profile-header'), 'grid-template-rows')).toBe('auto auto');
    expect(cssProperty(cssBlock('.me-profile__avatar'), 'grid-column')).toBe('1');
    expect(cssProperty(cssBlock('.me-profile__avatar'), 'grid-row')).toBe('1 / 3');
    expect(cssProperty(cssBlock('.me-profile__avatar'), 'align-self')).toBe('center');
    expect(cssProperty(cssBlock('.me-profile__identity'), 'grid-column')).toBe('2');
    expect(cssProperty(cssBlock('.me-profile__identity'), 'grid-row')).toBe('1');
    expect(cssProperty(cssBlock('.me-profile__identity'), 'align-self')).toBe('end');
    expect(cssProperty(cssBlock('.me-profile__status'), 'grid-row')).toBe('2');
    expect(cssProperty(cssBlock('.me-profile__status'), 'align-self')).toBe('start');
    expect(cssProperty(cssBlock('.me-profile__membership'), 'grid-column')).toBe('3');
    expect(cssProperty(cssBlock('.me-profile__membership'), 'grid-row')).toBe('1 / 3');
    expect(cssProperty(cssBlock('.me-profile__membership'), 'grid-template-columns')).toBe('minmax(0, 1fr)');
    expect(cssProperty(cssBlockWithinMedia('(max-width: 360px)', '.me-profile-header'), 'grid-template-columns'))
      .toBe('48px minmax(0, 1fr) 96px');
    expect(cssProperty(cssBlockWithinMedia('(max-width: 360px)', '.me-profile-header'), 'column-gap')).toBe('8px');
    expect(globalCss).not.toContain('.me-profile__logout');
  });

  it('uses the requested membership and checkin icon colors with a fixed permanent VIP badge', () => {
    expect(cssProperty(cssBlock('.membership-title__wheat'), 'background-color')).toBe('#f6c38a');
    expect(cssProperty(cssBlock('.me-checkin-heading__icon'), 'background-color')).toBe('#3f7d63');
    expect(cssProperty(cssBlock('.membership-permanent'), 'grid-template-columns')).toBe('84px minmax(0, 1fr)');
    expect(cssBlock('.membership-permanent')).not.toContain('border-left');
    expect(cssProperty(cssBlock('.membership-permanent__badge'), 'width')).toBe('84px');
    expect(cssProperty(cssBlock('.membership-permanent__badge'), 'background'))
      .toBe('linear-gradient(90deg, #d0aa74 0%, #fcefda 100%)');
  });

  it('keeps wallet links touch-sized with stable icons and a single-line visual value', () => {
    const balance = cssBlock('.me-profile__balance');
    const value = cssBlockForSelector('.me-profile__balance-value');
    const icon = cssBlock('.me-profile__currency-icon');

    expect(cssProperty(balance, 'min-height')).toBe('44px');
    expect(cssProperty(balance, 'width')).toBe('100%');
    expect(cssProperty(balance, 'border-radius')).toBe('6px');
    expect(cssProperty(balance, 'grid-template-columns')).toBe('26px minmax(0, 1fr) 16px');
    expect(cssProperty(value, 'min-width')).toBe('0');
    expect(cssProperty(value, 'overflow')).toBe('hidden');
    expect(cssProperty(value, 'text-overflow')).toBe('ellipsis');
    expect(cssProperty(value, 'white-space')).toBe('nowrap');
    expect(cssProperty(icon, 'width')).toBe('26px');
    expect(cssProperty(icon, 'height')).toBe('26px');
    expect(cssProperty(icon, 'object-fit')).toBe('contain');
  });

  it('uses the approved checkin surfaces and whole-card celebration geometry', () => {
    expect(cssProperty(cssBlockForSelector('.me-checkin-card'), 'border-radius')).toBe('16px');
    expect(cssProperty(cssBlockForSelector('.me-checkin-card'), 'background')).toBe('#fefcfc');
    expect(cssProperty(cssBlock('.me-checkin-stats > div'), 'background')).toBe('#fbf6f4');
    expect(cssBlock('.me-checkin-stats > div')).not.toContain('border-left');
    expect(cssProperty(cssBlock('.me-checkin-celebration'), 'border-radius')).toBe('16px');
    expect(cssProperty(cssBlock('.me-profile__status'), 'color')).toBe('#59655f');
    expect(cssProperty(cssBlock('.me-checkin-stats span'), 'color')).toBe('#59655f');
  });

  it('keeps controls touch sized and keyboard focus visible', () => {
    const focusSelectors = [
      '.me-checkin-action:focus-visible',
      '.me-invite-code button:focus-visible',
      '.me-invite-share:focus-visible',
      '.me-invite-rewards:focus-visible',
      '.me-invite-unavailable-state button:focus-visible',
      '.me-local-state button:focus-visible',
      '.me-inline-error button:focus-visible',
      '.me-profile__status button:focus-visible',
      '.me-profile__membership button:focus-visible',
      '.membership-plan-card:focus-visible',
      '.me-quick-link:focus-visible',
      '.invite-reward-dialog button:focus-visible'
    ];
    expect(cssProperty(cssBlockForSelector('.me-checkin-action'), 'min-height')).toBe('44px');
    expect(cssProperty(cssBlockForSelector('.me-invite-code button'), 'min-height')).toBe('44px');
    expect(cssProperty(cssBlockForSelector('.membership-plan-card'), 'min-height')).toBe('44px');
    expect(cssProperty(cssBlockForSelector('.me-quick-link'), 'min-height')).toBe('76px');
    focusSelectors.forEach((selector) => {
      expect(cssProperty(cssBlockForSelector(selector), 'outline')).toBe('3px solid #2f7564');
      expect(cssProperty(cssBlockForSelector(selector), 'outline-offset')).toBe('2px');
    });
    expect(cssProperty(cssBlockForSelector('.membership-plan-card:disabled'), 'opacity')).toBe('0.62');
    expect(cssProperty(cssBlockForSelector(".membership-plan-card[aria-busy='true']"), 'opacity')).toBe('1');
    expect(cssProperty(cssBlock('.membership-plan-card__status'), 'background'))
      .toBe('rgba(38, 52, 46, 0.78)');
    expect(cssProperty(cssBlock('.membership-plan-card__status'), 'color')).toBe('#fff');
  });

  it('uses coordinated invitation surfaces from the approved dashboard design', () => {
    expect(cssProperty(cssBlockForSelector('.me-invite-card'), 'border-radius')).toBe('16px');
    expect(cssProperty(cssBlockForSelector('.me-invite-card'), 'background')).toBe('#fefcfc');
    expect(cssProperty(cssBlock('.me-invite-code'), 'background')).toBe('#fff');
    expect(cssProperty(cssBlock('.me-invite-code'), 'border-radius')).toBe('10px');
  });

  it('disables motion-dependent snapping when reduced motion is requested', () => {
    expect(cssProperty(cssBlockWithinMedia('(prefers-reduced-motion: reduce)', '.membership-plans'), 'scroll-snap-type'))
      .toBe('none');
    expect(cssProperty(cssBlockWithinMedia('(prefers-reduced-motion: reduce)', '.me-checkin-skeleton span'), 'animation'))
      .toBe('none');
    expect(cssProperty(cssBlockWithinMedia('(prefers-reduced-motion: reduce)', '.me-checkin-celebration'), 'animation'))
      .toBe('none');
  });
});

describe('wallet detail visual design contract', () => {
  it('uses an unframed summary with a complete wrapping balance and stable loading geometry', () => {
    const shellBody = cssBlock('.app-shell:has(.wallet-detail-page) .app-shell__body');
    const summary = cssBlock('.wallet-detail__summary');
    const balance = cssBlock('.wallet-detail__balance');

    expect(cssProperty(shellBody, 'padding-bottom')).toBe('calc(16px + env(safe-area-inset-bottom))');
    expect(cssProperty(summary, 'border-bottom')).toBe('1px solid rgba(52, 70, 61, 0.16)');
    expect(summary).not.toContain('border-radius');
    expect(summary).not.toContain('box-shadow');
    expect(cssProperty(cssBlock('.wallet-detail__icon'), 'object-fit')).toBe('contain');
    expect(cssProperty(balance, 'overflow-wrap')).toBe('anywhere');
    expect(cssProperty(cssBlock('.wallet-detail__summary-skeleton'), 'min-height')).toBe('40px');
    expect(cssProperty(cssBlock('.wallet-detail__summary--with-action'), 'grid-template-columns'))
      .toBe('52px minmax(0, 1fr) auto');
    expect(cssProperty(cssBlock('.wallet-detail__recharge'), 'min-height')).toBe('44px');
  });

  it('keeps the recharge action in the three-column summary on mobile', () => {
    const mobileStyles = cssMediaBlocks('(max-width: 460px)').join('\n');

    expect(mobileStyles).not.toContain('.wallet-detail__summary--with-action');
    expect(mobileStyles).not.toContain('.wallet-detail__recharge');
  });

  it('uses full-width ledger separators and restrained transaction colors', () => {
    expect(cssProperty(cssBlock('.wallet-ledger-list'), 'width')).toBe('100%');
    expect(cssProperty(cssBlock('.wallet-ledger-row'), 'border-bottom')).toBe('1px solid rgba(52, 70, 61, 0.14)');
    expect(cssBlock('.wallet-ledger-row')).not.toContain('border-radius');
    expect(cssProperty(cssBlock('.wallet-ledger__amount--income'), 'color')).toBe('#25634f');
    expect(cssProperty(cssBlock('.wallet-ledger__amount--expense'), 'color')).toBe('#9a5a24');
    expect(cssProperty(cssBlockForSelector('.wallet-ledger__time'), 'color')).toBe('#6e7872');
    expect(cssProperty(cssBlockForSelector('.wallet-ledger__balance'), 'color')).toBe('#6e7872');
  });

  it('places ledger amounts beside descriptions and secondary details on the next row', () => {
    const positions = [
      ['.wallet-ledger__description', '1', '1'],
      ['.wallet-ledger__amount', '2', '1'],
      ['.wallet-ledger__time', '1', '2'],
      ['.wallet-ledger__balance', '2', '2']
    ];

    positions.forEach(([selector, column, row]) => {
      const block = cssBlockForSelector(selector);
      expect(cssProperty(block, 'grid-column')).toBe(column);
      expect(cssProperty(block, 'grid-row')).toBe(row);
    });
  });

  it('disables the wallet summary pulse for reduced motion', () => {
    expect(cssProperty(cssBlockWithinMedia('(prefers-reduced-motion: reduce)', '.wallet-detail__summary-skeleton-value'), 'animation'))
      .toBe('none');
  });

  it('keeps detail actions touch-sized with restrained corners', () => {
    ['.wallet-detail__balance-retry', '.wallet-ledger-more'].forEach((selector) => {
      const block = cssBlockForSelector(selector);
      expect(cssProperty(block, 'min-height')).toBe('44px');
      expect(Number.parseFloat(cssProperty(block, 'border-radius') ?? '')).toBeLessThanOrEqual(8);
    });
  });

  it('defines every existing wallet detail surface without card nesting', () => {
    [
      '.wallet-detail-page',
      '.wallet-detail__summary',
      '.wallet-detail__icon',
      '.wallet-detail__balance-label',
      '.wallet-detail__balance',
      '.wallet-detail__summary-skeleton',
      '.wallet-detail__ledgers',
      '.wallet-detail__ledger-title',
      '.wallet-ledger-list',
      '.wallet-ledger-row',
      '.wallet-ledger__description',
      '.wallet-ledger__time',
      '.wallet-ledger__amount',
      '.wallet-ledger__balance',
      '.wallet-ledger-state',
      '.wallet-ledger-more'
    ].forEach((selector) => expect(globalCss).toContain(selector));
  });
});
