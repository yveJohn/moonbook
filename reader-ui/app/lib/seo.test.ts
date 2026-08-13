import { describe, expect, it } from 'vitest';
import { booksSeo, renderTemplate, safeJsonLd, safeSeoConfig } from './seo';

describe('server SEO helpers', () => {
  it('renders selected book filters without exposing category codes', () => {
    const search = new URLSearchParams({
      keyword: '  月光  ',
      categoryCode: 'fantasy',
      subCategoryCode: 'system'
    });
    const result = booksSeo(
      {
        ...safeSeoConfig,
        booksTitleTemplate: '{keyword} - {categoryName} - {subCategoryName} - {siteName}'
      },
      search,
      [{ categoryCode: 'fantasy', categoryName: '奇幻' }],
      [{ categoryCode: 'system', categoryName: '系统流' }]
    );

    expect(result.title).toBe('月光 - 奇幻 - 系统流 - 月白书城');
    expect(result.title).not.toContain('fantasy');
  });

  it('cleans separators and truncates by Unicode code point', () => {
    expect(renderTemplate('{bookName} - {authorName} - {siteName}', {
      bookName: '😀😀😀', authorName: '', siteName: ''
    }, 2)).toBe('😀😀');
  });

  it('escapes script-closing markup in JSON-LD', () => {
    expect(safeJsonLd({ description: '</script><script>alert(1)</script>' }))
      .not.toContain('</script>');
  });
});
