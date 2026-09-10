import test from 'node:test'
import assert from 'node:assert/strict'
import { adminEnum, adminEnumColumn } from '../src/utils/adminEnums.js'

test('业务枚举按字段显示中文，复用列表与详情映射', () => {
  for (const [field, value, expected] of [
    ['rewardStage', 'first_recharge', '首充'], ['direction', 'expense', '支出'],
    ['coinType', 'bonus', '奖励币'], ['bizType', 'invite_register_reward', '邀请注册奖励'],
    ['bizType', 'wallet_adjustment', '钱包调整'], ['stage', 'discover', '发现'],
    ['module', 'novel_book_profile', '作品资料补全'], ['jobType', 'chapter_clean', '章节清洗'],
    ['contentSource', 'clean_result', '清洗结果'], ['contentType', 'advertisement', '广告'],
    ['source', 'legacy_book_author', '旧系统作品作者'], ['importMode', 'incremental', '增量导入'],
    ['mergeStrategy', 'source_thread', '按来源帖子合并'], ['roundingMode', 'CEILING', '向上取整']
  ]) {
    const row = Object.freeze({ [field]: value })
    assert.equal(adminEnum(field, value), expected)
    assert.equal(adminEnumColumn(row, { property: field }, value), expected)
    assert.equal(row[field], value)
  }
  assert.equal(adminEnum('productType', 'book'), '书籍')
  assert.equal(adminEnum('orderType', 'book'), '整书')
  assert.equal(adminEnum('orderType', 'buy_chapter'), '章节')
  assert.equal(adminEnum('bizType', 'bonus_expire'), '奖励币过期')
})

test('空值、未知值、中文和超长 ID 不会误翻译或丢失', () => {
  for (const value of [null, undefined, '']) assert.equal(adminEnum('stage', value), '-')
  for (const value of ['future_stage', '自定义阶段', '9223372036854775807', 'constructor', '__proto__']) {
    assert.equal(adminEnum('stage', value), value)
  }
  assert.equal(adminEnum('unknownField', 'book'), 'book')
  assert.equal(adminEnum('__proto__', 'constructor'), 'constructor')
  assert.equal(adminEnum('stage', 0), '0')
})
