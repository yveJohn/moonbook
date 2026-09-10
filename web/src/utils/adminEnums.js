// Display-only, field-scoped labels. API values and unknown identifiers remain intact.
const labels = {
  rewardStage: { register: '注册', first_recharge: '首充' },
  direction: { income: '收入', expense: '支出' },
  coinType: { recharge: '充值币', bonus: '奖励币' },
  bizType: {
    recharge: '充值', manual_recharge: '人工充值', mock_recharge: '模拟充值', epusdt_recharge: 'USDT充值',
    ad_free_purchase: '免广告购买', bonus_expire: '奖励币过期', checkin: '签到奖励', wallet_adjustment: '钱包调整', membership_purchase: '会员购买',
    chapter_purchase: '章节购买', book_purchase: '整书购买', invite_reward: '受邀奖励',
    invite_register_reward: '邀请注册奖励', invite_first_recharge_reward: '邀请首充奖励'
  },
  productType: { book: '书籍', chapter: '章节', membership: '会员', ad_free: '免广告' },
  orderType: { buy_book: '整书', buy_chapter: '章节', buy_membership: '会员', buy_ad_free: '免广告', book: '整书', chapter: '章节', membership: '会员', mock_recharge: '模拟充值' },
  module: { novel: '小说', novel_book_profile: '作品资料补全' },
  jobType: { chapter_summary: '章节摘要', chapter_clean: '章节清洗', forum_import: '论坛导入', txt_import: '文本导入', object_gc: '对象回收', generate: '生成作品资料' },
  stage: { discover: '发现', thread: '帖子', chapter: '章节', import: '导入', quality: '质量' },
  contentType: { novel: '小说正文', chat: '闲聊', forum_noise: '论坛杂项', advertisement: '广告', mixed: '混合内容', unknown: '未知' },
  contentSource: { clean_result: '清洗结果', original: '原始正文' },
  source: { native: '本系统创建', legacy_dict: '旧系统字典', legacy_category: '旧系统分类', legacy_relation: '旧系统分类关联', legacy_author: '旧系统作者', legacy_book_author: '旧系统作品作者', legacy_book: '旧系统作品' },
  importMode: { create: '新建作品', incremental: '增量导入' },
  mergeStrategy: { source_thread: '按来源帖子合并', same_title: '按同名作品合并' },
  roundingMode: { CEILING: '向上取整', FLOOR: '向下取整', HALF_UP: '四舍五入', HALF_EVEN: '四舍六入五成双', UP: '远离零取整', DOWN: '向零取整', HALF_DOWN: '五舍六入', UNNECESSARY: '无需舍入' }
}

export function adminEnum(field, value) {
  if (value === null || value === undefined || value === '') return '-'
  const dictionary = Object.hasOwn(labels, field) ? labels[field] : null
  return dictionary && Object.hasOwn(dictionary, value) ? dictionary[value] : String(value)
}

export function adminEnumColumn(_row, column, value) {
  return adminEnum(column.property, value)
}
