import { http, unwrap, unwrapPage } from './http';
import type {
  ReaderBookDetail,
  ReaderBookCatalogItem,
  ReaderBookLike,
  ReaderBookQuery,
  ReaderBookSummary,
  ReaderBookshelf,
  ReaderCategory,
  ReaderCheckinStatus,
  ReaderChapterContent,
  ReaderChapterPurchaseResult,
  ReaderChapterSummary,
  ReaderEntitlements,
  ReaderFeedback,
  ReaderHistoryUpdatePayload,
  ReaderInviteDashboard,
  ReaderLikedBook,
  ReaderLoginResult,
  ReaderLongValue,
  ReaderOrder,
  ReaderPageResult,
  ReaderPasswordChangePayload,
  ReaderPreference,
  ReaderPreferencePayload,
  ReaderProfile,
  ReaderProduct,
  ReaderReadingHistory,
  ReaderRegisterPayload,
  ReaderRechargeCatalog,
  ReaderRechargeOrder,
  ReaderRechargeQuote,
  ReaderWallet,
  ReaderWalletLedger,
  ReaderWalletLedgerQuery
} from '../types/reader';

export type { ReaderOrder } from '../types/reader';

export async function listFeaturedBooks(): Promise<ReaderBookSummary[]> {
  return unwrap(await http.get('/reader/books/featured'));
}

export async function queryBooks(params: ReaderBookQuery): Promise<ReaderPageResult<ReaderBookCatalogItem>> {
  return unwrapPage(await http.get('/reader/books', { params }));
}

export async function listRandomBooks(): Promise<ReaderBookCatalogItem[]> {
  return unwrap(await http.get('/reader/books/random'));
}

export async function listCategories(): Promise<ReaderCategory[]> {
  return unwrap(await http.get('/reader/books/categories'));
}

export async function listSubCategories(): Promise<ReaderCategory[]> {
  return unwrap(await http.get('/reader/books/sub-categories'));
}

export async function getBook(bookId: string): Promise<ReaderBookDetail> {
  return unwrap(await http.get(`/reader/books/${bookId}`));
}

export async function listBookChapters(bookId: string): Promise<ReaderChapterSummary[]> {
  return unwrap(await http.get(`/reader/books/${bookId}/chapters`));
}

export async function getChapter(chapterId: string): Promise<ReaderChapterContent> {
  return unwrap(await http.get(`/reader/chapters/${chapterId}`));
}

export async function registerReader(payload: ReaderRegisterPayload): Promise<ReaderLoginResult> {
  return unwrap(await http.post('/reader/auth/register', payload));
}

export async function loginReader(username: string, password: string): Promise<ReaderLoginResult> {
  return unwrap(await http.post('/reader/auth/login', { username, password }));
}

export async function logoutReader(): Promise<void> {
  unwrap(await http.post('/reader/auth/logout'));
}

export async function getReaderProfile(): Promise<ReaderProfile> {
  return unwrap(await http.get('/reader/auth/profile'));
}

export async function changeReaderPassword(payload: ReaderPasswordChangePayload, tokenSnapshot: string): Promise<void> {
  if (typeof tokenSnapshot !== 'string' || tokenSnapshot.length === 0) {
    throw new Error('Reader token is required');
  }
  unwrap(await http.put('/reader/auth/password', payload, {
    headers: { Authorization: `Bearer ${tokenSnapshot}` }
  }));
}

export async function getWallet(): Promise<ReaderWallet> {
  return unwrap(await http.get('/reader/me/wallet'));
}

export async function queryWalletLedgers(
  params: ReaderWalletLedgerQuery
): Promise<ReaderPageResult<ReaderWalletLedger>> {
  return unwrapPage(await http.get('/reader/me/wallet/ledgers', { params }));
}

export async function getRechargeCatalog(): Promise<ReaderRechargeCatalog> {
  return unwrap(await http.get('/reader/recharge/products'));
}

export async function quoteRecharge(diamondAmount: string): Promise<ReaderRechargeQuote> {
  return unwrap(await http.post('/reader/recharge/quote', { diamondAmount }));
}

export async function createRechargeOrder(payload: {
  productId?: string;
  customDiamondAmount?: string;
  requestId: string;
}): Promise<ReaderRechargeOrder> {
  return unwrap(await http.post('/reader/me/recharge/orders', payload));
}

export async function getRechargeOrder(orderId: string): Promise<ReaderRechargeOrder> {
  return unwrap(await http.get(`/reader/me/recharge/orders/${orderId}`));
}

export async function getCheckinStatus(): Promise<ReaderCheckinStatus> {
  return unwrap(await http.get('/reader/me/checkin/status'));
}

export async function ensureInviteDashboard(): Promise<ReaderInviteDashboard> {
  return unwrap(await http.post('/reader/me/invite/code'));
}

export async function checkin(): Promise<ReaderCheckinStatus> {
  return unwrap(await http.post('/reader/me/checkin'));
}

export async function getEntitlements(): Promise<ReaderEntitlements> {
  return unwrap(await http.get('/reader/me/entitlements'));
}

export async function listMembershipProducts(): Promise<ReaderProduct[]> {
  return unwrap(await http.get('/reader/products/membership'));
}

export async function buyMembership(productId: string, requestId: string): Promise<ReaderOrder> {
  return unwrap(await http.post('/reader/me/orders/membership', { productId, requestId }));
}

export async function buyChapter(
  chapterId: string,
  expectedPrice: ReaderLongValue,
  requestId: string
): Promise<ReaderChapterPurchaseResult> {
  return unwrap(await http.post('/reader/me/orders/chapter', { chapterId, expectedPrice, requestId }));
}

export async function buyBook(bookId: string, expectedPrice: ReaderLongValue): Promise<ReaderOrder> {
  return unwrap(await http.post('/reader/me/orders/book', { bookId, expectedPrice }));
}

export async function listBookshelf(): Promise<ReaderBookshelf[]> {
  return unwrap(await http.get('/reader/me/bookshelf'));
}

export async function addBookshelf(bookId: string): Promise<ReaderBookshelf> {
  return unwrap(await http.post(`/reader/me/bookshelf/${bookId}`));
}

export async function removeBookshelf(bookId: string): Promise<boolean> {
  return unwrap(await http.delete(`/reader/me/bookshelf/${bookId}`));
}

export async function likeBook(bookId: string): Promise<ReaderBookLike> {
  return unwrap(await http.post(`/reader/me/likes/${bookId}`));
}

export async function unlikeBook(bookId: string): Promise<ReaderBookLike> {
  return unwrap(await http.delete(`/reader/me/likes/${bookId}`));
}

export async function listLikedBooks(): Promise<ReaderLikedBook[]> {
  return unwrap(await http.get('/reader/me/likes'));
}

export async function createFeedback(content: string): Promise<ReaderFeedback> {
  return unwrap(await http.post('/reader/me/feedbacks', { content }));
}

export async function queryFeedbacks(pageNum = 1, pageSize = 10): Promise<ReaderPageResult<ReaderFeedback>> {
  return unwrapPage(await http.get('/reader/me/feedbacks', { params: { pageNum, pageSize } }));
}

export async function listHistory(): Promise<ReaderReadingHistory[]> {
  return unwrap(await http.get('/reader/me/history'));
}

export async function getBookHistory(bookId: string): Promise<ReaderReadingHistory | null> {
  return unwrap(await http.get(`/reader/me/history/${bookId}`));
}

export async function updateHistory(bookId: string, payload: ReaderHistoryUpdatePayload): Promise<ReaderReadingHistory> {
  return unwrap(await http.put(`/reader/me/history/${bookId}`, payload));
}

export async function getPreference(): Promise<ReaderPreference> {
  return unwrap(await http.get('/reader/me/preference'));
}

export async function savePreference(payload: ReaderPreferencePayload): Promise<ReaderPreference> {
  return unwrap(await http.put('/reader/me/preference', payload));
}
