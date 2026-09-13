import test from 'node:test'
import assert from 'node:assert/strict'
import { actionControls } from './actionState'
test('unknown never offers resend', () => {
  assert.deepEqual(actionControls({id:'a', state:'unknown', digest:'d', canApprove:true, canExecute:true}),
    {approve:false, execute:false, retry:false})
})
test('permission and lifecycle both required', () => {
  assert.equal(actionControls({id:'a', state:'authorized', digest:'d', canApprove:false, canExecute:false}).execute, false)
})
