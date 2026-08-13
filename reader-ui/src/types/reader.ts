export type ReaderTheme = 'cream' | 'night' | 'green';
export type ReaderMode = 'scroll' | 'page';
export type ReaderIndentMode = 'indent' | 'none';
export type ReaderFontFamily = 'system' | 'serif' | 'hei' | 'kai';
export type ReaderLongValue = number | string;
export type ReaderRechargeStatus = 'creating' | 'pending' | 'gateway_unknown' | 'create_failed' | 'superseded' | 'expired' | 'callback_exception' | 'paid';

export interface ReaderRechargeProduct {
  id: string;
  productName: string;
  diamondAmount: string;
  priceUsdt: string;
  saleStatus: string;
  sortOrder: number;
}

export interface ReaderRechargeCatalog {
  products: ReaderRechargeProduct[];
  customEnabled: boolean;
  diamondsPerUsdt: string;
  minDiamondAmount: string;
  maxDiamondAmount: string;
}

export interface ReaderRechargeQuote {
  diamondAmount: string;
  priceUsdt: string;
}

export interface ReaderRechargeOrder {
  id: string;
  orderNo: string;
  readerId: string;
  sourceType: 'preset' | 'custom';
  productId: string | null;
  diamondAmount: string;
  priceUsdt: string;
  provider: string;
  currency: string;
  token: string;
  network: string;
  gatewayTradeId: string | null;
  actualAmount: string | null;
  receiveAddress: string | null;
  paymentUrl: string | null;
  blockTransactionId: string | null;
  status: ReaderRechargeStatus;
  gatewayStatus: number | null;
  walletLedgerId: string | null;
  expireTime: string | null;
  paidTime: string | null;
  failureCode: string | null;
  failureMessage: string | null;
  createTime: string;
  updateTime: string;
}
const bookChargeModes = ['word_charge', 'membership_only', 'login_free', 'fixed_price'] as const;
export type BookChargeMode = (typeof bookChargeModes)[number];
export type BookChargeModeWire = string | null;

const readerAccessReasons = [
  'login_required',
  'book_owned',
  'chapter_owned',
  'membership',
  'login_free',
  'membership_required',
  'book_purchase_required',
  'chapter_purchase_required',
  'free_chapter',
  'unsupported_mode'
] as const;
export type ReaderAccessReason = (typeof readerAccessReasons)[number];
export type ReaderAccessReasonWire = string | null;

export function isBookChargeMode(value: unknown): value is BookChargeMode {
  return typeof value === 'string' && (bookChargeModes as readonly string[]).includes(value);
}

export function isReaderAccessReason(value: unknown): value is ReaderAccessReason {
  return typeof value === 'string' && (readerAccessReasons as readonly string[]).includes(value);
}

export interface ReaderBookSubCategory {
  categoryCode: string;
  categoryName: string;
  sort: number;
}

export interface ReaderBookSummary {
  bookId: string;
  bookName: string;
  authorName: string;
  bookDesc: string;
  categoryCode: string;
  categoryName: string;
  subCategories?: ReaderBookSubCategory[];
  bookStatus: string;
  wordCount: number;
  likeCount?: number;
  commentCount?: number;
  favoriteCount?: number;
  lastChapterId: string;
  lastChapterName: string;
  lastChapterUpdateTime: string;
  featured: number;
  featuredNote: string;
  productStatus: ReaderBookProductStatus;
}

export type ReaderBookSort = 'recent' | 'popular' | 'words';

export interface ReaderBookCatalogItem {
  bookId: string;
  bookName: string;
  categoryCode: string;
  categoryName: string;
  subCategories?: ReaderBookSubCategory[];
  wordCount: number;
  likeCount: number;
  lastChapterUpdateTime: string;
}

export interface ReaderProduct {
  id: string;
  productType: string;
  targetId: string | null;
  productName: string;
  priceCoin: ReaderLongValue;
  allowBonusCoin: number;
  durationDays: number | null;
  saleStatus: string;
  sortOrder: number;
  remark: string;
  createTime: string;
  updateTime: string;
}

export interface ReaderBookProductStatus {
  bookId: string;
  chargeMode: BookChargeModeWire;
  readable: boolean;
  accessReason: ReaderAccessReasonWire;
  entitled: boolean;
  purchased: boolean;
  membershipEntitled: boolean;
  purchasable: boolean;
  productId: string | null;
  productName: string | null;
  priceCoin: ReaderLongValue | null;
  saleStatus: string | null;
  product: ReaderProduct | null;
}

export interface ReaderEntitlements {
  readerId: string;
  adFreeActive?: boolean;
  adFreeExpireTime?: string | null;
  membershipActive?: boolean;
  membershipExpireTime?: string | null;
  membershipPermanent?: boolean;
  bookIds: string[];
}

export interface ReaderProfile {
  readerId: string;
  username: string;
  nickname: string;
  status: string;
}

export interface ReaderLoginResult {
  accessToken: string;
  expireIn: number;
  reader: ReaderProfile;
}

export interface ReaderCategory {
  categoryCode: string;
  categoryName: string;
  bookCount?: number;
}

export interface ReaderReadingHistory {
  historyId: string;
  bookId: string;
  chapterId: string;
  chapterNo: number;
  chapterName: string;
  positionType: string;
  positionValue: number;
  progressPercent: string | number;
  lastReadTime: string;
}

export type ReaderFeedbackStatus = 'pending' | 'replied';

export interface ReaderFeedback {
  id: string;
  content: string;
  status: ReaderFeedbackStatus;
  replyContent: string | null;
  replyTime: string | null;
  createTime: string;
}

export interface ReaderBookDetail extends ReaderBookSummary {
  visitCount: number;
  liked: boolean;
  inBookshelf: boolean;
  readingHistory: ReaderReadingHistory | null;
}

export interface ReaderChapterSummary {
  chapterId: string;
  bookId: string;
  chapterNo: number;
  chapterName: string;
  wordCount: number;
  updateTime: string;
  accessStatus: ReaderChapterAccess;
  productStatus: ReaderBookProductStatus;
}

export interface ReaderChapterAccess {
  bookId: string;
  chapterId: string;
  chargeMode: BookChargeModeWire;
  readable: boolean;
  accessReason: ReaderAccessReasonWire;
  membershipEntitled: boolean;
  bookPurchased: boolean;
  chapterPurchased: boolean;
  purchasable: boolean;
  chapterWordCount: number | null;
  pricingWordUnit: number | null;
  pricingCoinUnit: ReaderLongValue | null;
  chapterPrice: ReaderLongValue | null;
}

export interface ReaderOrder {
  id: string | null;
  orderNo: string | null;
  readerId: string;
  orderType: string;
  productId: string | null;
  productType: string;
  targetId: string | null;
  bookIdSnapshot: string | null;
  productNameSnapshot: string | null;
  priceCoinSnapshot: ReaderLongValue | null;
  chapterWordCountSnapshot: number | null;
  pricingWordUnitSnapshot: number | null;
  pricingCoinUnitSnapshot: ReaderLongValue | null;
  rechargeCoinAmount: ReaderLongValue;
  bonusCoinAmount: ReaderLongValue;
  status: string;
  idempotencyKey: string;
  remark: string | null;
  operatorId: string | null;
  paidTime: string | null;
  createTime: string | null;
  updateTime: string | null;
}

export interface ReaderChapterQuote {
  chapterId: string;
  bookId: string;
  wordCount: number;
  wordUnit: number;
  coinUnit: ReaderLongValue;
  priceCoin: ReaderLongValue;
}

export interface ReaderChapterPurchaseResult {
  purchaseStatus: 'paid' | 'already_owned' | 'free' | 'quote_changed';
  quote: ReaderChapterQuote;
  order: ReaderOrder | null;
}

export interface ReaderChapterContent {
  chapterId: string;
  bookId: string;
  chapterNo: number;
  chapterName: string;
  bookName: string;
  content: string;
  prevChapterId: string;
  nextChapterId: string;
}

export interface ReaderBookshelf {
  bookshelfId: string;
  bookId: string;
  bookName: string;
  authorName: string;
  lastChapterId: string;
  lastChapterName: string;
  lastReadTime: string;
}

export interface ReaderBookLike {
  likeId: string | null;
  bookId: string;
  liked: boolean;
  likeCount: number;
}

export interface ReaderLikedBook {
  likeId: string;
  bookId: string;
  bookName: string;
  authorName: string | null;
  bookDesc: string | null;
  categoryCode: string | null;
  categoryName: string | null;
  wordCount: number;
  likeCount: number;
  likedAt: string;
}

export interface ReaderPreference {
  preferenceId: string;
  fontSize: number;
  lineHeight: string | number;
  theme: ReaderTheme;
  readingMode: ReaderMode;
}

export interface ReaderWallet {
  readerId: string;
  rechargeCoinBalance: ReaderLongValue;
  bonusCoinBalance: ReaderLongValue;
  totalRechargeCoinIncome: ReaderLongValue;
  totalBonusCoinIncome: ReaderLongValue;
  totalRechargeCoinExpense: ReaderLongValue;
  totalBonusCoinExpense: ReaderLongValue;
  expiringBonusCoin: ReaderLongValue;
}

export type ReaderWalletCoinType = 'recharge' | 'bonus';
export type ReaderWalletDirection = 'income' | 'expense';

export interface ReaderWalletLedger {
  id: string;
  readerId: string;
  ledgerNo: string;
  bizType: string;
  bizId: string | null;
  orderNo: string | null;
  direction: ReaderWalletDirection;
  coinType: ReaderWalletCoinType;
  amount: ReaderLongValue;
  balanceBefore: ReaderLongValue;
  balanceAfter: ReaderLongValue;
  remark: string | null;
  createTime: string;
}

export interface ReaderWalletLedgerQuery {
  coinType: ReaderWalletCoinType;
  pageNum: number;
  pageSize: number;
}

export interface ReaderCheckinStatus {
  todayChecked: boolean;
  continuousDays: number;
  todayRewardCoin: ReaderLongValue;
  rewardRandom?: boolean;
  rewardText?: string;
  checkinAvailable?: boolean;
  unavailableReason?: string;
}

export interface ReaderInviteRewardRecord {
  id: string;
  rewardStage: string;
  rewardCoin: ReaderLongValue;
  grantTime: string;
  remark?: string | null;
}

export interface ReaderInviteDashboard {
  readerId: string;
  inviteCode: string | null;
  inviteCodeAvailable: boolean;
  inviteCodeUnavailableReason?: string | null;
  shareTextTemplate: string;
  registerRewardCoin?: ReaderLongValue;
  firstRechargeRewardCoin?: ReaderLongValue;
  invitedCount: ReaderLongValue;
  totalRewardCoin: ReaderLongValue;
  rewards: ReaderInviteRewardRecord[];
}

export interface ReaderBookQuery {
  keyword?: string;
  categoryCode?: string;
  subCategoryCode?: string;
  sort?: ReaderBookSort;
  pageNum?: number;
  pageSize?: number;
}

export interface ReaderPageResult<T> {
  rows: T[];
  total: number;
}

export interface ReaderRegisterPayload {
  username: string;
  password: string;
  nickname: string;
  inviteCode: string;
}

export interface ReaderPasswordChangePayload {
  currentPassword: string;
  newPassword: string;
  confirmPassword: string;
}

export interface ReaderHistoryUpdatePayload {
  chapterId: string;
  chapterNo: number;
  positionType: ReaderMode;
  positionValue: number;
  progressPercent: string;
}

export interface ReaderPreferencePayload {
  fontSize: number;
  lineHeight: string;
  theme: ReaderTheme;
  readingMode: ReaderMode;
}

export interface ReaderLocalPreference extends ReaderPreferencePayload {
  marginSize: number;
  indentMode: ReaderIndentMode;
  fontFamily: ReaderFontFamily;
}
