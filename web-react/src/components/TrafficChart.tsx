import { useEffect, useRef } from 'react'
import { Box, useTheme } from '@mui/material'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import type { TrafficHistoryItem } from '@/api/traffic'

// Register only what we actually render: a line chart with axis/tooltip/legend.
// Pulling the umbrella `echarts` package would ship every chart type and
// renderer (~1MB) for a single time-series plot.
echarts.use([LineChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer])

interface Props {
  items: TrafficHistoryItem[]
  loading?: boolean
  height?: number
}

function bytesToHuman(n: number) {
  if (n === 0) return '0'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let v = n, u = 0
  while (v >= 1024 && u < units.length - 1) { v /= 1024; u++ }
  return `${v.toFixed(2)} ${units[u]}`
}

export default function TrafficChart({ items, height = 360 }: Props) {
  const ref = useRef<HTMLDivElement | null>(null)
  const chartRef = useRef<echarts.ECharts | null>(null)
  const theme = useTheme()
  const md = theme.palette.md

  useEffect(() => {
    if (!ref.current) return
    chartRef.current = echarts.init(ref.current, null, { renderer: 'canvas' })
    const onResize = () => chartRef.current?.resize()
    window.addEventListener('resize', onResize)
    return () => {
      window.removeEventListener('resize', onResize)
      chartRef.current?.dispose()
      chartRef.current = null
    }
  }, [])

  useEffect(() => {
    if (!chartRef.current) return
    const dates = items.map(i => i.date)
    const up = items.map(i => i.up_bytes)
    const down = items.map(i => i.down_bytes)
    const total = items.map(i => i.total_bytes)
    chartRef.current.setOption({
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'axis',
        valueFormatter: (v: number | string) => bytesToHuman(Number(v)),
      },
      legend: {
        data: ['Up', 'Down', 'Total'],
        textStyle: { color: md.onSurfaceVariant },
        top: 4,
      },
      grid: { left: 56, right: 24, top: 40, bottom: 32 },
      xAxis: {
        type: 'category',
        data: dates,
        axisLabel: { color: md.onSurfaceVariant },
        axisLine: { lineStyle: { color: md.outlineVariant } },
      },
      yAxis: {
        type: 'value',
        axisLabel: {
          color: md.onSurfaceVariant,
          formatter: (v: number) => bytesToHuman(v),
        },
        splitLine: { lineStyle: { color: md.outlineVariant, opacity: 0.5 } },
      },
      series: [
        { name: 'Up', type: 'line', data: up, smooth: true, itemStyle: { color: md.primary }, areaStyle: { opacity: 0.1 } },
        { name: 'Down', type: 'line', data: down, smooth: true, itemStyle: { color: md.tertiary }, areaStyle: { opacity: 0.1 } },
        { name: 'Total', type: 'line', data: total, smooth: true, itemStyle: { color: md.secondary } },
      ],
    }, true)
  }, [items, md])

  return <Box ref={ref} sx={{ width: '100%', height }} />
}
