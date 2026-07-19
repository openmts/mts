import assert from 'node:assert/strict'
import test from 'node:test'
import {
  backupScheduleCatalog,
  edgeHttpsCatalog,
  productionCatalog,
  titlesById,
} from './checklistCatalog.ts'

test('productionCatalog localizes and marks done', () => {
  const zh = productionCatalog(['https-edge'], 'zh')
  const en = productionCatalog(['https-edge'], 'en')
  const rowZh = zh.find((x) => x.id === 'https-edge')
  const rowEn = en.find((x) => x.id === 'https-edge')
  assert.ok(rowZh?.done)
  assert.ok(rowEn?.done)
  assert.match(rowZh!.title, /HTTPS|TLS|边缘/)
  assert.equal(rowEn!.title, 'Edge HTTPS / TLS')
  assert.ok(en.every((x) => x.title && x.detail))
})

test('edge and schedule catalogs expose english titles', () => {
  const edge = edgeHttpsCatalog(['tls-terminate'], 'en')
  assert.equal(edge.find((x) => x.id === 'tls-terminate')?.title, 'Edge TLS termination')
  const sched = backupScheduleCatalog(['define-rpo-rto'], 'en')
  assert.equal(sched.find((x) => x.id === 'define-rpo-rto')?.title, 'Define RPO / RTO')
  assert.ok(titlesById(sched)['define-rpo-rto'])
})
