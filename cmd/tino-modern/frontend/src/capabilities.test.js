import test from 'node:test'
import assert from 'node:assert/strict'
import { normalizeCapabilities } from './capabilities.js'

test('normaliza o contrato JSON emitido pelo backend Go', () => {
  const result = normalizeCapabilities({
    modules: [{ id: 'flow' }],
    roles: [{ id: 'admin', modules: ['flow'] }],
    operators: ['5573999999999'],
    workspaceRoot: 'C:\\Dados',
  })
  assert.equal(result.modules[0].id, 'flow')
  assert.deepEqual(result.roles[0].modules, ['flow'])
  assert.deepEqual(result.operators, ['5573999999999'])
  assert.equal(result.workspaceRoot, 'C:\\Dados')
})

test('mantém compatibilidade defensiva com nomes antigos', () => {
  const result = normalizeCapabilities({ Modules: [], Roles: [], WorkspaceRoot: '' })
  assert.deepEqual(result, { modules: [], roles: [], operators: [], workspaceRoot: '' })
})
