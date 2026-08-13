import type { AxiosAdapter } from 'axios';
import { afterEach, describe, expect, expectTypeOf, it } from 'vitest';
import type {
  ReaderBookProductStatus,
  ReaderBookSummary,
  ReaderChapterAccess,
  ReaderChapterPurchaseResult,
  ReaderChapterSummary,
  ReaderLikedBook,
  ReaderLongValue,
  ReaderOrder,
  ReaderProduct,
  ReaderWalletCoinType,
  ReaderWalletDirection,
  ReaderWalletLedger,
  ReaderWalletLedgerQuery
} from '../types/reader';
import { http } from './http';
import {
  buyBook,
  buyChapter,
  changeReaderPassword,
  ensureInviteDashboard,
  listRandomBooks,
  listSubCategories,
  listLikedBooks,
  queryBooks,
  queryWalletLedgers
} from './reader';

const originalAdapter = http.defaults.adapter;

afterEach(() => {
  http.defaults.adapter = originalAdapter;
  window.localStorage.clear();
});

describe('reader book category API', () => {
  it('loads sub-categories and submits the sub-category query as a string', async () => {
    const requests: Array<{ url?: string; params?: unknown }> = [];
    const adapter: AxiosAdapter = async (config) => {
      requests.push({ url: config.url, params: config.params });
      return {
        config,
        data: config.url?.endsWith('/sub-categories')
          ? { code: 200, data: [{ categoryCode: 'system', categoryName: '系统' }] }
          : { code: 200, rows: [], total: 0 },
        headers: {},
        status: 200,
        statusText: 'OK'
      };
    };
    http.defaults.adapter = adapter;

    const categories = await listSubCategories();
    await queryBooks({ subCategoryCode: 'system', sort: 'popular', pageNum: 1, pageSize: 10 });

    expect(categories).toEqual([{ categoryCode: 'system', categoryName: '系统' }]);
    expect(requests).toEqual([
      { url: '/reader/books/sub-categories', params: undefined },
      { url: '/reader/books', params: { subCategoryCode: 'system', sort: 'popular', pageNum: 1, pageSize: 10 } }
    ]);
  });

  it('loads a random batch from the dedicated endpoint', async () => {
    const requests: string[] = [];
    http.defaults.adapter = async (config) => {
      requests.push(config.url || '');
      return {
        config,
        data: { code: 200, data: [{ bookId: '9223372036854775807', bookName: '随机作品' }] },
        headers: {},
        status: 200,
        statusText: 'OK'
      };
    };

    const result = await listRandomBooks();

    expect(requests).toEqual(['/reader/books/random']);
    expect(result[0]?.bookId).toBe('9223372036854775807');
  });
});

describe('reader invite API', () => {
  it('posts the idempotent invite endpoint and preserves string ids', async () => {
    let method = '';
    let url = '';
    let body: unknown;
    const adapter: AxiosAdapter = async (config) => {
      method = config.method || '';
      url = config.url || '';
      body = config.data;
      return {
        config,
        data: {
          code: 200,
          data: {
            readerId: '9223372036854775807',
            inviteCode: 'MBABCDEFGH',
            inviteCodeAvailable: true,
            registerRewardCoin: '9007199254740993',
            firstRechargeRewardCoin: '9223372036854775806',
            invitedCount: '9007199254740992',
            totalRewardCoin: '9223372036854775807',
            rewards: [{
              id: '9223372036854775806',
              rewardStage: 'register',
              rewardCoin: '9223372036854775806',
              grantTime: '2026-07-10 10:00:00'
            }]
          }
        },
        headers: {},
        status: 200,
        statusText: 'OK'
      };
    };
    http.defaults.adapter = adapter;

    const result = await ensureInviteDashboard();

    expect(method).toBe('post');
    expect(url).toBe('/reader/me/invite/code');
    expect(body).toBeUndefined();
    expect(result.readerId).toBe('9223372036854775807');
    expect(result.rewards[0].id).toBe('9223372036854775806');
    expect(result.registerRewardCoin).toBe('9007199254740993');
    expect(typeof result.registerRewardCoin).toBe('string');
    expect(result.firstRechargeRewardCoin).toBe('9223372036854775806');
    expect(typeof result.firstRechargeRewardCoin).toBe('string');
    expect(result.invitedCount).toBe('9007199254740992');
    expect(typeof result.invitedCount).toBe('string');
    expect(result.totalRewardCoin).toBe('9223372036854775807');
    expect(typeof result.totalRewardCoin).toBe('string');
    expect(result.rewards[0].rewardCoin).toBe('9223372036854775806');
    expect(typeof result.rewards[0].rewardCoin).toBe('string');
  });
});

describe('reader wallet API', () => {
  it('gets paged wallet ledgers without coercing long ids or amounts', async () => {
    let method = '';
    let url = '';
    let params: unknown;
    const adapter: AxiosAdapter = async (config) => {
      method = config.method || '';
      url = config.url || '';
      params = config.params;
      return {
        config,
        data: {
          code: 200,
          rows: [{
            id: '9223372036854775807',
            readerId: '9223372036854775806',
            ledgerNo: 'ledger-1',
            bizType: 'checkin',
            bizId: '9223372036854775805',
            orderNo: null,
            direction: 'income',
            coinType: 'bonus',
            amount: '9223372036854775804',
            balanceBefore: '9223372036854775803',
            balanceAfter: '9223372036854775802',
            remark: '签到奖励',
            createTime: '2026-07-14 10:00:00'
          }],
          total: 21
        },
        headers: {},
        status: 200,
        statusText: 'OK'
      };
    };
    http.defaults.adapter = adapter;

    const result = await queryWalletLedgers({ coinType: 'bonus', pageNum: 2, pageSize: 20 });

    expect(method).toBe('get');
    expect(url).toBe('/reader/me/wallet/ledgers');
    expect(params).toEqual({ coinType: 'bonus', pageNum: 2, pageSize: 20 });
    expect(result.rows).toHaveLength(1);
    expect(result.total).toBe(21);
    expect(result.rows[0].id).toBe('9223372036854775807');
    expect(result.rows[0].readerId).toBe('9223372036854775806');
    expect(result.rows[0].bizId).toBe('9223372036854775805');
    expect(result.rows[0].amount).toBe('9223372036854775804');
    expect(result.rows[0].balanceBefore).toBe('9223372036854775803');
    expect(result.rows[0].balanceAfter).toBe('9223372036854775802');
    expect(typeof result.rows[0].id).toBe('string');
    expect(typeof result.rows[0].readerId).toBe('string');
    expect(typeof result.rows[0].bizId).toBe('string');
    expect(typeof result.rows[0].amount).toBe('string');
    expect(typeof result.rows[0].balanceBefore).toBe('string');
    expect(typeof result.rows[0].balanceAfter).toBe('string');
  });

  it('models wallet ledger ids, amounts, filters, and enums precisely', () => {
    expectTypeOf<ReaderWalletLedger['id']>().toEqualTypeOf<string>();
    expectTypeOf<ReaderWalletLedger['readerId']>().toEqualTypeOf<string>();
    expectTypeOf<ReaderWalletLedger['bizId']>().toEqualTypeOf<string | null>();
    expectTypeOf<ReaderWalletLedger['amount']>().toEqualTypeOf<ReaderLongValue>();
    expectTypeOf<ReaderWalletLedger['balanceBefore']>().toEqualTypeOf<ReaderLongValue>();
    expectTypeOf<ReaderWalletLedger['balanceAfter']>().toEqualTypeOf<ReaderLongValue>();
    expectTypeOf<ReaderWalletLedger['coinType']>().toEqualTypeOf<ReaderWalletCoinType>();
    expectTypeOf<ReaderWalletLedger['direction']>().toEqualTypeOf<ReaderWalletDirection>();
    expectTypeOf<ReaderWalletLedgerQuery>().toEqualTypeOf<{
      coinType: ReaderWalletCoinType;
      pageNum: number;
      pageSize: number;
    }>();
  });
});

describe('reader account API', () => {
  it('gets liked books without coercing long ids', async () => {
    let method = '';
    let url = '';
    let body: unknown;
    const adapter: AxiosAdapter = async (config) => {
      method = config.method || '';
      url = config.url || '';
      body = config.data;
      return {
        config,
        data: {
          code: 200,
          data: [{
            likeId: '9223372036854775807',
            bookId: '9223372036854775806',
            bookName: '测试作品',
            authorName: '测试作者',
            bookDesc: '测试简介',
            categoryCode: 'fiction',
            categoryName: '小说',
            wordCount: 120000,
            likeCount: 88,
            likedAt: '2026-07-13 10:00:00'
          }]
        },
        headers: {},
        status: 200,
        statusText: 'OK'
      };
    };
    http.defaults.adapter = adapter;

    const result = await listLikedBooks();

    expect(method).toBe('get');
    expect(url).toBe('/reader/me/likes');
    expect(body).toBeUndefined();
    expect(result[0].likeId).toBe('9223372036854775807');
    expect(result[0].bookId).toBe('9223372036854775806');
    expect(typeof result[0].likeId).toBe('string');
    expect(typeof result[0].bookId).toBe('string');
  });

  it('puts the complete password change payload', async () => {
    let method = '';
    let url = '';
    let body: unknown;
    let authorization: unknown;
    window.localStorage.setItem('readerToken', 'token-B');
    const adapter: AxiosAdapter = async (config) => {
      method = config.method || '';
      url = config.url || '';
      body = typeof config.data === 'string' ? JSON.parse(config.data) : config.data;
      authorization = config.headers?.Authorization;
      return {
        config,
        data: { code: 200, data: null },
        headers: {},
        status: 200,
        statusText: 'OK'
      };
    };
    http.defaults.adapter = adapter;

    const result = await changeReaderPassword({
      currentPassword: 'old-password',
      newPassword: 'new-password',
      confirmPassword: 'new-password'
    }, 'token-A');

    expect(method).toBe('put');
    expect(url).toBe('/reader/auth/password');
    expect(body).toEqual({
      currentPassword: 'old-password',
      newPassword: 'new-password',
      confirmPassword: 'new-password'
    });
    expect(authorization).toBe('Bearer token-A');
    expect(result).toBeUndefined();
  });

  it('rejects an empty password-change token before dispatch', async () => {
    let dispatched = false;
    http.defaults.adapter = async (config) => {
      dispatched = true;
      return { config, data: { code: 200, data: null }, headers: {}, status: 200, statusText: 'OK' };
    };

    await expect(changeReaderPassword({
      currentPassword: 'old-password',
      newPassword: 'new-password',
      confirmPassword: 'new-password'
    }, '')).rejects.toThrow('Reader token is required');

    expect(dispatched).toBe(false);
  });

  it('rejects a password change business error from a successful HTTP response', async () => {
    const adapter: AxiosAdapter = async (config) => ({
      config,
      data: { code: 400, msg: '当前密码错误', data: null },
      headers: {},
      status: 200,
      statusText: 'OK'
    });
    http.defaults.adapter = adapter;

    await expect(changeReaderPassword({
      currentPassword: 'wrong-password',
      newPassword: 'new-password',
      confirmPassword: 'new-password'
    }, 'reader-token')).rejects.toThrow('当前密码错误');
  });

  it('models liked book ids as strings', () => {
    expectTypeOf<ReaderLikedBook['likeId']>().toEqualTypeOf<string>();
    expectTypeOf<ReaderLikedBook['bookId']>().toEqualTypeOf<string>();
    expectTypeOf<ReaderLikedBook['authorName']>().toEqualTypeOf<string | null>();
    expectTypeOf<ReaderLikedBook['bookDesc']>().toEqualTypeOf<string | null>();
    expectTypeOf<ReaderLikedBook['categoryCode']>().toEqualTypeOf<string | null>();
    expectTypeOf<ReaderLikedBook['categoryName']>().toEqualTypeOf<string | null>();
  });
});

describe('reader purchase API', () => {
  it('posts and unwraps a chapter purchase without coercing long values', async () => {
    let method = '';
    let url = '';
    let body: unknown;
    const adapter: AxiosAdapter = async (config) => {
      method = config.method || '';
      url = config.url || '';
      body = typeof config.data === 'string' ? JSON.parse(config.data) : config.data;
      return {
        config,
        data: {
          code: 200,
          data: {
            purchaseStatus: 'paid',
            quote: {
              chapterId: '9223372036854775807',
              bookId: '9223372036854775806',
              wordCount: 2500,
              wordUnit: 1000,
              coinUnit: '9223372036854775804',
              priceCoin: '9223372036854775803'
            },
            order: {
              id: '9223372036854775802',
              orderNo: 'chapter-order-1',
              readerId: '9223372036854775801',
              orderType: 'buy_chapter',
              productId: null,
              productType: 'chapter',
              targetId: '9223372036854775807',
              bookIdSnapshot: '9223372036854775806',
              productNameSnapshot: '测试作品 - 测试章节',
              priceCoinSnapshot: '9223372036854775803',
              chapterWordCountSnapshot: 2500,
              pricingWordUnitSnapshot: 1000,
              pricingCoinUnitSnapshot: '9223372036854775804',
              rechargeCoinAmount: '9223372036854775800',
              bonusCoinAmount: '9223372036854775799',
              status: 'paid',
              idempotencyKey: 'buy_chapter:9223372036854775801:9223372036854775807:request-1',
              remark: null,
              operatorId: null,
              paidTime: '2026-07-11 20:00:00',
              createTime: '2026-07-11 20:00:00',
              updateTime: '2026-07-11 20:00:00'
            }
          }
        },
        headers: {},
        status: 200,
        statusText: 'OK'
      };
    };
    http.defaults.adapter = adapter;

    const result = await buyChapter('9223372036854775807', '9223372036854775803', 'request-1');

    expect(method).toBe('post');
    expect(url).toBe('/reader/me/orders/chapter');
    expect(body).toEqual({
      chapterId: '9223372036854775807',
      expectedPrice: '9223372036854775803',
      requestId: 'request-1'
    });
    expect(result.quote.coinUnit).toBe('9223372036854775804');
    expect(result.quote.priceCoin).toBe('9223372036854775803');
    expect(result.order?.id).toBe('9223372036854775802');
    expect(result.order?.targetId).toBe('9223372036854775807');
    expect(result.order?.priceCoinSnapshot).toBe('9223372036854775803');
    expect(result.order?.rechargeCoinAmount).toBe('9223372036854775800');
    expect(result.order?.bonusCoinAmount).toBe('9223372036854775799');
  });

  it('posts and unwraps a book purchase without coercing long values', async () => {
    let method = '';
    let url = '';
    let body: unknown;
    const adapter: AxiosAdapter = async (config) => {
      method = config.method || '';
      url = config.url || '';
      body = typeof config.data === 'string' ? JSON.parse(config.data) : config.data;
      return {
        config,
        data: {
          code: 200,
          data: {
            id: '9223372036854775805',
            orderNo: 'book-order-1',
            readerId: '9223372036854775804',
            orderType: 'buy_book',
            productId: '9223372036854775803',
            productType: 'book',
            targetId: '9223372036854775806',
            bookIdSnapshot: null,
            productNameSnapshot: '测试作品',
            priceCoinSnapshot: '9223372036854775802',
            chapterWordCountSnapshot: null,
            pricingWordUnitSnapshot: null,
            pricingCoinUnitSnapshot: null,
            rechargeCoinAmount: '9223372036854775801',
            bonusCoinAmount: '9223372036854775800',
            status: 'paid',
            idempotencyKey: 'buy_book:9223372036854775804:9223372036854775806',
            remark: null,
            operatorId: null,
            paidTime: '2026-07-11 20:00:00',
            createTime: '2026-07-11 20:00:00',
            updateTime: '2026-07-11 20:00:00'
          }
        },
        headers: {},
        status: 200,
        statusText: 'OK'
      };
    };
    http.defaults.adapter = adapter;

    const result = await buyBook('9223372036854775806', '9223372036854775807');

    expect(method).toBe('post');
    expect(url).toBe('/reader/me/orders/book');
    expect(body).toEqual({
      bookId: '9223372036854775806',
      expectedPrice: '9223372036854775807'
    });
    expect(result.id).toBe('9223372036854775805');
    expect(result.readerId).toBe('9223372036854775804');
    expect(result.productId).toBe('9223372036854775803');
    expect(result.targetId).toBe('9223372036854775806');
    expect(result.priceCoinSnapshot).toBe('9223372036854775802');
    expect(result.rechargeCoinAmount).toBe('9223372036854775801');
    expect(result.bonusCoinAmount).toBe('9223372036854775800');
  });

  it('models authoritative access and nullable purchase fields as present properties', () => {
    expectTypeOf<ReaderBookSummary['productStatus']>().toEqualTypeOf<ReaderBookProductStatus>();
    expectTypeOf<ReaderChapterSummary['accessStatus']>().toEqualTypeOf<ReaderChapterAccess>();
    expectTypeOf<ReaderChapterSummary['productStatus']>().toEqualTypeOf<ReaderBookProductStatus>();
    expectTypeOf<ReaderChapterAccess['chapterPrice']>().toEqualTypeOf<ReaderLongValue | null>();
    expectTypeOf<ReaderBookProductStatus['productId']>().toEqualTypeOf<string | null>();
    expectTypeOf<ReaderChapterPurchaseResult['order']>().toEqualTypeOf<ReaderOrder | null>();
    expectTypeOf<ReaderOrder['productId']>().toEqualTypeOf<string | null>();
    expectTypeOf<ReaderOrder['targetId']>().toEqualTypeOf<string | null>();
    expectTypeOf<ReaderOrder['remark']>().toEqualTypeOf<string | null>();
    expectTypeOf<ReaderProduct['targetId']>().toEqualTypeOf<string | null>();
    expectTypeOf<ReaderProduct['durationDays']>().toEqualTypeOf<number | null>();
  });
});
