<template>
  <main class="operations-dashboard">
    <header class="dashboard-header">
      <div>
        <p class="dashboard-kicker">读者运营</p>
        <h1>运营概览</h1>
        <p class="dashboard-meta">
          <span v-if="overview">更新于 {{ adminDateTime(generatedAt) }}</span>
          <span v-else>汇总用户增长与活跃趋势</span>
        </p>
      </div>
      <el-tooltip content="刷新运营数据" placement="bottom">
        <el-button
          :icon="Refresh"
          circle
          aria-label="刷新运营数据"
          :loading="refreshing"
          @click="loadOverview"
        />
      </el-tooltip>
    </header>

    <el-alert
      v-if="error && overview"
      class="refresh-alert"
      type="warning"
      :closable="false"
      show-icon
      title="最新数据获取失败，当前仍显示上一次结果"
    />

    <section v-if="loading" class="metrics-grid" aria-label="运营数据加载中">
      <div v-for="item in 4" :key="item" class="metric-panel skeleton-panel">
        <el-skeleton animated :rows="2" />
      </div>
    </section>

    <section v-else-if="error && !overview" class="initial-error">
      <el-empty description="运营数据暂时无法加载">
        <el-button type="primary" @click="loadOverview">重新加载</el-button>
      </el-empty>
    </section>

    <template v-else-if="overview">
      <section class="metrics-grid" aria-label="核心运营指标">
        <article v-for="metric in primaryMetrics" :key="metric.label" class="metric-panel">
          <div class="metric-heading">
            <span>{{ metric.label }}</span>
            <component :is="metric.icon" class="metric-icon" />
          </div>
          <strong>{{ formatCount(metric.value) }}</strong>
          <p :class="['metric-context', metric.comparison?.direction]">
            {{ metric.context }}
          </p>
        </article>
      </section>

      <section class="period-strip" aria-label="周期统计">
        <div v-for="item in periodMetrics" :key="item.label" class="period-item">
          <span>{{ item.label }}</span>
          <strong>{{ formatCount(item.value) }}</strong>
        </div>
      </section>

      <section class="trend-section">
        <div class="section-heading">
          <div>
            <h2>近 30 天趋势</h2>
            <p>按 Asia/Kuala_Lumpur 自然日统计</p>
          </div>
          <div class="tracking-note">
            活跃数据起始于 {{ overview.activity.activityTrackingStartDate }}
          </div>
        </div>
        <Chart class="trend-chart" height="360px" :option="chartOption" />
      </section>
    </template>
  </main>
</template>

<script setup>
import { adminDateTime } from '@/utils/adminDisplay'
  import { computed, onMounted, ref } from 'vue'
  import { Calendar, Refresh, TrendCharts, User } from '@element-plus/icons-vue'
  import Chart from '@/components/charts/index.vue'
  import { getDashboardOverview } from '@/api/dashboard'
  import { buildTrendSeries, compareCounts, formatCount } from './dashboardMetrics'

  defineOptions({ name: 'Dashboard' })

  const overview = ref(null)
  const loading = ref(true)
  const refreshing = ref(false)
  const error = ref(false)

  const generatedAt = computed(() => overview.value?.generatedAt || '')

  const primaryMetrics = computed(() => {
    const growth = overview.value.growth
    const activity = overview.value.activity
    const newComparison = compareCounts(growth.todayNewUserCount, growth.yesterdayNewUserCount)
    const activeComparison = compareCounts(activity.todayActiveUserCount, activity.yesterdayActiveUserCount)
    return [
      { label: '累计读者', value: growth.totalUserCount, context: `近 30 天新增 ${formatCount(growth.last30DaysNewUserCount)}`, icon: User },
      { label: '今日新增', value: growth.todayNewUserCount, context: newComparison.text, comparison: newComparison, icon: TrendCharts },
      { label: '今日活跃', value: activity.todayActiveUserCount, context: activeComparison.text, comparison: activeComparison, icon: Calendar },
      { label: '近 30 天活跃', value: activity.last30DaysActiveUserCount, context: '周期内去重读者', icon: User }
    ]
  })

  const periodMetrics = computed(() => {
    const growth = overview.value.growth
    const activity = overview.value.activity
    return [
      { label: '昨日新增', value: growth.yesterdayNewUserCount },
      { label: '昨日活跃', value: activity.yesterdayActiveUserCount },
      { label: '近 7 天新增', value: growth.last7DaysNewUserCount },
      { label: '近 7 天活跃', value: activity.last7DaysActiveUserCount }
    ]
  })

  const chartOption = computed(() => {
    const trend = buildTrendSeries(overview.value?.dailyTrend)
    return {
      animationDuration: 350,
      color: ['#2563eb', '#15956b'],
      grid: { left: 46, right: 22, top: 42, bottom: 42 },
      legend: { top: 0, left: 0, itemWidth: 18, itemHeight: 3, textStyle: { color: '#64748b' }, data: ['新增读者', '活跃读者'] },
      tooltip: { trigger: 'axis', valueFormatter: (value) => `${formatCount(value)} 人` },
      xAxis: {
        type: 'category', boundaryGap: false, data: trend.dates,
        axisLine: { lineStyle: { color: '#dce3ec' } }, axisTick: { show: false },
        axisLabel: { color: '#64748b', formatter: (value) => value.slice(5) }
      },
      yAxis: {
        type: 'value', minInterval: 1, axisLine: { show: false }, axisTick: { show: false },
        axisLabel: { color: '#64748b' }, splitLine: { lineStyle: { color: '#e8edf3', type: 'dashed' } }
      },
      series: [
        { name: '新增读者', type: 'line', data: trend.newUsers, smooth: 0.25, showSymbol: false, lineStyle: { width: 2 } },
        { name: '活跃读者', type: 'line', data: trend.activeUsers, smooth: 0.25, showSymbol: false, lineStyle: { width: 2 } }
      ]
    }
  })

  const loadOverview = async () => {
    overview.value ? (refreshing.value = true) : (loading.value = true)
    error.value = false
    try {
      const response = await getDashboardOverview()
      overview.value = response.data
    } catch {
      error.value = true
    } finally {
      loading.value = false
      refreshing.value = false
    }
  }

  onMounted(loadOverview)
</script>

<style lang="scss" scoped>
  .operations-dashboard {
    min-height: 100%;
    overflow: auto;
    padding: 24px;
    color: var(--el-text-color-primary);
    background: #f4f6f8;
  }
  .dashboard-header, .section-heading {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 16px;
  }
  .dashboard-kicker { margin: 0 0 5px; color: #16745a; font-size: 13px; font-weight: 600; }
  h1 { margin: 0; font-size: 26px; line-height: 1.25; font-weight: 650; letter-spacing: 0; }
  .dashboard-meta, .section-heading p { margin: 6px 0 0; color: #718096; font-size: 13px; }
  .refresh-alert { margin-top: 16px; }
  .metrics-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin-top: 22px; }
  .metric-panel { min-height: 132px; padding: 18px; border: 1px solid #e1e6ec; border-radius: 6px; background: #fff; }
  .metric-heading { display: flex; align-items: center; justify-content: space-between; color: #667085; font-size: 13px; }
  .metric-icon { width: 18px; height: 18px; color: #16745a; }
  .metric-panel strong { display: block; margin-top: 16px; font-size: 28px; line-height: 1; font-variant-numeric: tabular-nums; letter-spacing: 0; }
  .metric-context { margin: 13px 0 0; color: #718096; font-size: 12px; }
  .metric-context.up { color: #13795b; }
  .metric-context.down { color: #b5473c; }
  .period-strip { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); margin-top: 12px; border: 1px solid #e1e6ec; border-radius: 6px; background: #fff; }
  .period-item { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; padding: 15px 18px; border-right: 1px solid #e8ecf1; }
  .period-item:last-child { border-right: 0; }
  .period-item span { color: #718096; font-size: 13px; }
  .period-item strong { font-size: 18px; font-variant-numeric: tabular-nums; }
  .trend-section { margin-top: 12px; padding: 20px 20px 10px; border: 1px solid #e1e6ec; border-radius: 6px; background: #fff; }
  .section-heading h2 { margin: 0; font-size: 17px; line-height: 1.4; letter-spacing: 0; }
  .tracking-note { padding-top: 3px; color: #718096; font-size: 12px; }
  .trend-chart { margin-top: 22px; }
  .initial-error { display: grid; min-height: 420px; place-items: center; }
  .skeleton-panel { display: flex; align-items: center; }
  @media (max-width: 980px) {
    .metrics-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .period-strip { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .period-item:nth-child(2) { border-right: 0; }
    .period-item:nth-child(-n+2) { border-bottom: 1px solid #e8ecf1; }
  }
  @media (max-width: 560px) {
    .operations-dashboard { padding: 16px; }
    h1 { font-size: 23px; }
    .metrics-grid { grid-template-columns: 1fr; margin-top: 18px; }
    .metric-panel { min-height: 122px; }
    .period-strip { grid-template-columns: 1fr; }
    .period-item { border-right: 0; border-bottom: 1px solid #e8ecf1; }
    .period-item:last-child { border-bottom: 0; }
    .section-heading { flex-direction: column; gap: 2px; }
    .trend-section { padding-inline: 14px; }
  }
  :global(.dark) .operations-dashboard { background: #11161d; }
  :global(.dark) .metric-panel, :global(.dark) .period-strip, :global(.dark) .trend-section { border-color: #2a3340; background: #1a2029; }
  :global(.dark) .period-item { border-color: #2a3340; }
</style>
