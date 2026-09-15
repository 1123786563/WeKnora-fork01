import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./index.vue', import.meta.url), 'utf8')

test('platform shell excludes the global Settings instance on the settings child route', () => {
  assert.match(
    source,
    /<Settings\s+v-if="route\.name !== 'settings'"\s*\/>/,
    'the settings child route must own its single Settings instance',
  )
})
