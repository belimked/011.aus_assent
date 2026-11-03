import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { BarChart, LineChart, PieChart, HeatmapChart, GaugeChart, FunnelChart, RadarChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent, VisualMapComponent, DataZoomComponent, TitleComponent, PolarComponent, RadarComponent } from 'echarts/components'
import VChart from 'vue-echarts'

use([CanvasRenderer, BarChart, LineChart, PieChart, HeatmapChart, GaugeChart, FunnelChart, RadarChart, GridComponent, TooltipComponent, LegendComponent, VisualMapComponent, DataZoomComponent, TitleComponent, PolarComponent, RadarComponent])
export default VChart
