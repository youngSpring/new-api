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
import { useState } from 'react'

import { useMediaQuery } from '@/hooks/use-media-query'

import loginVideo from '../assets/relayhub-login-loop.mp4'

export function SignInVisual() {
  const canAnimate = useMediaQuery(
    '(min-width: 64rem) and (prefers-reduced-motion: no-preference)'
  )
  const [videoFailed, setVideoFailed] = useState(false)

  return (
    <div className='relative isolate mx-4 mt-6 flex min-h-32 items-center overflow-hidden rounded-2xl bg-[#080e1d] bg-[radial-gradient(ellipse_at_24%_35%,#24458a_0%,transparent_66%)] px-6 sm:mx-8 lg:sticky lg:top-0 lg:m-0 lg:h-svh lg:rounded-none lg:px-0'>
      {canAnimate && !videoFailed && (
        <>
          <video
            src={loginVideo}
            className='pointer-events-none absolute inset-0 size-full object-cover'
            aria-hidden='true'
            autoPlay
            muted
            loop
            playsInline
            disablePictureInPicture
            onError={() => setVideoFailed(true)}
          />
          <div className='pointer-events-none absolute inset-x-0 bottom-0 h-28 bg-linear-to-t from-black/65 to-transparent' />
        </>
      )}
    </div>
  )
}
