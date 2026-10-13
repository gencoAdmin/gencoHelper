declare module 'echarts' {
 export interface ChartInstance { setOption(option: unknown, replace?: boolean): void; resize(): void; dispose(): void }
 export function init(element: HTMLElement): ChartInstance
}
