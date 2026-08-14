<template>
  <VCharts
    v-if="renderChart"
    :option="options"
    :autoresize="autoResize"
    :style="{ width, height }"
  />
</template>

<script setup>
  import { ref, nextTick } from 'vue'
  import VCharts from 'vue-echarts'
  import { use } from 'echarts/core'
  import { CanvasRenderer } from 'echarts/renderers'
  import { LineChart } from 'echarts/charts'
  import { GraphicComponent, GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
  import { useWindowResize } from '@/hooks/use-windows-resize'

  use([CanvasRenderer, LineChart, GraphicComponent, GridComponent, LegendComponent, TooltipComponent])

  defineProps({
    options: {
      type: Object,
      default() {
        return {}
      }
    },
    autoResize: {
      type: Boolean,
      default: true
    },
    width: {
      type: String,
      default: '100%'
    },
    height: {
      type: String,
      default: '100%'
    }
  })
  const renderChart = ref(false)
  nextTick(() => {
    renderChart.value = true
  })
  useWindowResize(() => {
    renderChart.value = false
    nextTick(() => {
      renderChart.value = true
    })
  })
</script>

<style scoped lang="less"></style>
