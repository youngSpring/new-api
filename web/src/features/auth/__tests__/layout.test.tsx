/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { render } from '@testing-library/react'
import type { ReactNode } from 'react'
import { expect, test, vi } from 'vitest'

import { AuthLayout } from '../auth-layout'

vi.mock('@tanstack/react-router', () => ({
  Link: (props: { children?: ReactNode; className?: string }) => (
    <a className={props.className} href='#'>
      {props.children}
    </a>
  ),
}))

vi.mock('@/hooks/use-system-config', () => ({
  useSystemConfig: () => ({
    systemName: 'RelayHub',
    logo: '/logo.png',
    loading: false,
  }),
}))

test('a visual layout caps the form panel at 560px on large screens', () => {
  const { container } = render(
    <AuthLayout visual={<div data-testid='visual' />}>
      <p>form</p>
    </AuthLayout>
  )
  const root = container.firstElementChild
  if (!root) throw new Error('Expected the layout root')

  expect(root.className).toContain(
    'lg:grid-cols-[minmax(0,1fr)_minmax(0,560px)]'
  )
  expect(root.className).not.toContain('1.12fr')
})

test('a layout without a visual stays a single full-height panel', () => {
  const { container } = render(
    <AuthLayout>
      <p>form</p>
    </AuthLayout>
  )
  const root = container.firstElementChild
  if (!root) throw new Error('Expected the layout root')

  expect(root.className).toContain('h-svh')
  expect(root.className).not.toContain('grid-cols')
})
