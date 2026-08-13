import type { ReaderBookProductStatus, ReaderBookSummary, ReaderCategory } from '../types/reader';

function loginFreeProductStatus(bookId: string): ReaderBookProductStatus {
  return {
    bookId,
    chargeMode: 'login_free',
    readable: true,
    accessReason: 'login_free',
    entitled: false,
    purchased: false,
    membershipEntitled: false,
    purchasable: false,
    productId: null,
    productName: null,
    priceCoin: null,
    saleStatus: null,
    product: null
  };
}

/**
 * Demo data used as a fallback on the home page when the backend is
 * unavailable or returns nothing, so the layout always has content to show.
 */
export const sampleFeaturedBooks: ReaderBookSummary[] = [
  {
    bookId: 'demo-1',
    bookName: '月落长安',
    authorName: '青衫',
    bookDesc: '一座旧城，一段被月色藏起的往事。',
    categoryCode: 'history',
    categoryName: '历史',
    bookStatus: '1',
    wordCount: 326000,
    likeCount: 1820,
    lastChapterId: 'demo-1-c',
    lastChapterName: '第八十二章 城南旧雪',
    lastChapterUpdateTime: '2026-06-20 21:10:00',
    featured: 1,
    featuredNote: '编辑推荐',
    productStatus: loginFreeProductStatus('demo-1')
  },
  {
    bookId: 'demo-2',
    bookName: '山海拾遗录',
    authorName: '林深',
    bookDesc: '行走于山海之间，记下那些不该被遗忘的名字。',
    categoryCode: 'fantasy',
    categoryName: '玄幻',
    bookStatus: '1',
    wordCount: 512000,
    likeCount: 4365,
    lastChapterId: 'demo-2-c',
    lastChapterName: '第一百零三章 归墟',
    lastChapterUpdateTime: '2026-06-24 09:30:00',
    featured: 1,
    featuredNote: '本周热读',
    productStatus: loginFreeProductStatus('demo-2')
  },
  {
    bookId: 'demo-3',
    bookName: '雨夜便利店',
    authorName: '苏晚',
    bookDesc: '凌晨三点的便利店，收留每一个无处可去的人。',
    categoryCode: 'urban',
    categoryName: '都市',
    bookStatus: '1',
    wordCount: 188000,
    likeCount: 932,
    lastChapterId: 'demo-3-c',
    lastChapterName: '第四十一章 第二十七个客人',
    lastChapterUpdateTime: '2026-06-26 23:05:00',
    featured: 1,
    featuredNote: '',
    productStatus: loginFreeProductStatus('demo-3')
  },
  {
    bookId: 'demo-4',
    bookName: '星图之下',
    authorName: '陈未',
    bookDesc: '当人类抬头数星，星也在数着人类。',
    categoryCode: 'scifi',
    categoryName: '科幻',
    bookStatus: '1',
    wordCount: 274000,
    likeCount: 1287,
    lastChapterId: 'demo-4-c',
    lastChapterName: '第六十五章 第二信使',
    lastChapterUpdateTime: '2026-06-25 14:48:00',
    featured: 1,
    featuredNote: '',
    productStatus: loginFreeProductStatus('demo-4')
  },
  {
    bookId: 'demo-5',
    bookName: '青梅煮酒',
    authorName: '温故',
    bookDesc: '少年时未说出口的话，多年后都酿成了酒。',
    categoryCode: 'romance',
    categoryName: '言情',
    bookStatus: '1',
    wordCount: 142000,
    likeCount: 2640,
    lastChapterId: 'demo-5-c',
    lastChapterName: '第三十章 后来',
    lastChapterUpdateTime: '2026-06-22 18:20:00',
    featured: 1,
    featuredNote: '高分完结',
    productStatus: loginFreeProductStatus('demo-5')
  }
];

export const sampleCategories: ReaderCategory[] = [
  { categoryCode: 'fantasy', categoryName: '玄幻', bookCount: 128 },
  { categoryCode: 'urban', categoryName: '都市', bookCount: 96 },
  { categoryCode: 'history', categoryName: '历史', bookCount: 54 },
  { categoryCode: 'scifi', categoryName: '科幻', bookCount: 47 },
  { categoryCode: 'romance', categoryName: '言情', bookCount: 73 },
  { categoryCode: 'mystery', categoryName: '悬疑', bookCount: 38 }
];
