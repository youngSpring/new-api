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
import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { SignInVisual } from '../sign-in-visual'

function allowBackgroundVideo(allowed: boolean) {
  vi.spyOn(window, 'matchMedia').mockImplementation((media) => ({
    matches:
      allowed &&
      media ===
        '(min-width: 64rem) and (prefers-reduced-motion: no-preference)',
    media,
    onchange: null,
    addListener: () => undefined,
    removeListener: () => undefined,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    dispatchEvent: () => false,
  }))
}

afterEach(() => vi.restoreAllMocks())

test('the wide-screen video autoplays silently with no controls or brand mark', () => {
  allowBackgroundVideo(true)
  const { container } = render(<SignInVisual />)
  const video = container.querySelector('video')
  if (!video) throw new Error('Expected the background video')
  expect(video.autoplay).toBe(true)
  expect(video.loop).toBe(true)
  expect(video.muted).toBe(true)
  expect(screen.queryByRole('button')).not.toBeInTheDocument()
  expect(container.querySelector('img')).not.toBeInTheDocument()
})

test('a narrow viewport or reduced motion skips the background video', () => {
  allowBackgroundVideo(false)
  const { container } = render(<SignInVisual />)

  expect(container.querySelector('video')).not.toBeInTheDocument()
  expect(screen.queryByRole('button')).not.toBeInTheDocument()
})

test('a failed video unmounts and leaves no controls behind', () => {
  allowBackgroundVideo(true)
  const { container } = render(<SignInVisual />)
  const video = container.querySelector('video')
  if (!video) throw new Error('Expected the background video')

  fireEvent.error(video)
  expect(container.querySelector('video')).not.toBeInTheDocument()
  expect(screen.queryByRole('button')).not.toBeInTheDocument()
})
