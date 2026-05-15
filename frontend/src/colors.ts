// 表主题色：dot 用于导航、border 用于卡片四周边框（hover 加深）、halo 用于 hover 时的柔光晕。

export interface ColorSet {
  dot: string
  border: string  // 默认 1px 淡彩边框 + hover 时同色加深
  halo: string    // hover 时四周对称柔光（无粗环）
  picker: string  // 移动端表切换选中态
}

export const colorMap: Record<string, ColorSet> = {
  slate:   { dot: 'bg-slate-400',   border: 'border-slate-400/40 hover:border-slate-400/80',   halo: 'hover:shadow-[0_0_24px_0_rgb(100_116_139_/_0.3)]', picker: 'border-slate-400/50 bg-slate-500/10 text-slate-950 dark:text-slate-50' },
  blue:    { dot: 'bg-blue-500',    border: 'border-blue-500/40 hover:border-blue-500/80',    halo: 'hover:shadow-[0_0_24px_0_rgb(59_130_246_/_0.35)]', picker: 'border-blue-500/45 bg-blue-500/10 text-blue-950 dark:text-blue-50' },
  emerald: { dot: 'bg-emerald-500', border: 'border-emerald-500/40 hover:border-emerald-500/80', halo: 'hover:shadow-[0_0_24px_0_rgb(16_185_129_/_0.35)]', picker: 'border-emerald-500/45 bg-emerald-500/10 text-emerald-950 dark:text-emerald-50' },
  orange:  { dot: 'bg-orange-500',  border: 'border-orange-500/40 hover:border-orange-500/80', halo: 'hover:shadow-[0_0_24px_0_rgb(249_115_22_/_0.35)]', picker: 'border-orange-500/45 bg-orange-500/10 text-orange-950 dark:text-orange-50' },
  sky:     { dot: 'bg-sky-500',     border: 'border-sky-500/40 hover:border-sky-500/80',     halo: 'hover:shadow-[0_0_24px_0_rgb(14_165_233_/_0.35)]', picker: 'border-sky-500/45 bg-sky-500/10 text-sky-950 dark:text-sky-50' },
  teal:    { dot: 'bg-teal-500',    border: 'border-teal-500/40 hover:border-teal-500/80',    halo: 'hover:shadow-[0_0_24px_0_rgb(20_184_166_/_0.35)]', picker: 'border-teal-500/45 bg-teal-500/10 text-teal-950 dark:text-teal-50' },
  amber:   { dot: 'bg-amber-500',   border: 'border-amber-500/40 hover:border-amber-500/80', halo: 'hover:shadow-[0_0_24px_0_rgb(245_158_11_/_0.35)]', picker: 'border-amber-500/45 bg-amber-500/10 text-amber-950 dark:text-amber-50' },
  rose:    { dot: 'bg-rose-500',    border: 'border-rose-500/40 hover:border-rose-500/80',   halo: 'hover:shadow-[0_0_24px_0_rgb(244_63_94_/_0.35)]', picker: 'border-rose-500/45 bg-rose-500/10 text-rose-950 dark:text-rose-50' },
  violet:  { dot: 'bg-violet-500',  border: 'border-violet-500/40 hover:border-violet-500/80', halo: 'hover:shadow-[0_0_24px_0_rgb(139_92_246_/_0.35)]', picker: 'border-violet-500/45 bg-violet-500/10 text-violet-950 dark:text-violet-50' },
  claude:  { dot: 'bg-[#d97757]',    border: 'border-[#d97757]/45 hover:border-[#d97757]/85', halo: 'hover:shadow-[0_0_24px_0_rgb(217_119_87_/_0.35)]', picker: 'border-[#d97757]/45 bg-[#d97757]/10 text-stone-950 dark:text-stone-50' },
}

// 头像背景：低饱和深色单色，按字符串哈希挑一个。
const avatarPalette = [
  'bg-zinc-800',
  'bg-stone-700',
  'bg-slate-700',
  'bg-neutral-700',
  'bg-blue-800',
  'bg-indigo-800',
  'bg-teal-800',
  'bg-emerald-800',
  'bg-amber-800',
  'bg-rose-800',
]

export function avatarColor(seed: string): string {
  let h = 0
  for (let i = 0; i < seed.length; i++) {
    h = (h * 31 + seed.charCodeAt(i)) >>> 0
  }
  return avatarPalette[h % avatarPalette.length]
}

export function getColorSet(color: string): ColorSet {
  return colorMap[color] ?? colorMap.slate
}
