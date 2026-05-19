import { execFileSync } from 'node:child_process'

const files = execFileSync('git', ['ls-files', '--', 'frontend/src'], {
  cwd: new URL('../..', import.meta.url),
  encoding: 'utf8',
})
  .split('\n')
  .filter(Boolean)
  .map((path) => path.replace(/^frontend\//, ''))

if (files.length === 0) {
  console.error('No frontend/src files found in Git index; case-path check cannot run.')
  process.exit(1)
}

const byLowerPath = new Map()
for (const path of files) {
  const parts = path.split('/')
  for (let i = 1; i <= parts.length; i += 1) {
    const current = parts.slice(0, i).join('/')
    const key = current.toLowerCase()
    const paths = byLowerPath.get(key) ?? new Set()
    paths.add(current)
    byLowerPath.set(key, paths)
  }
}

const caseConflicts = [...byLowerPath.values()].filter((paths) => paths.size > 1)
const acmeDirConflicts = files.filter((path) => path.startsWith('src/components/Acme/'))

if (caseConflicts.length > 0 || acmeDirConflicts.length > 0) {
  if (caseConflicts.length > 0) {
    console.error('Found case-conflicting frontend paths:')
    for (const paths of caseConflicts) {
      for (const path of paths) {
        console.error(`  ${path}`)
      }
    }
  }
  if (acmeDirConflicts.length > 0) {
    console.error('Use src/components/acme/ for ACME child modules, not src/components/Acme/:')
    for (const path of acmeDirConflicts) {
      console.error(`  ${path}`)
    }
  }
  process.exit(1)
}
