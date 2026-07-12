import { useState } from 'react'
import { enableJojaRoute } from '../../api'
import { errorMessage } from '../../core/helpers'
import { submitAndWaitForPlayerCommand } from './player-command-results'

const JOJA_CONFIRM_TEXT = 'IRREVERSIBLY_ENABLE_JOJA_RUN'

type ServerJojaOptions = {
  isAdmin: boolean
  isRunning: boolean
}

export function useServerJoja({ isAdmin, isRunning }: ServerJojaOptions) {
  const [jojaOpen, setJojaOpen] = useState(false)
  const [jojaConfirmInput, setJojaConfirmInput] = useState('')
  const [jojaBusy, setJojaBusy] = useState(false)
  const [jojaMessage, setJojaMessage] = useState<string | null>(null)
  const [jojaError, setJojaError] = useState(false)

  function openJojaConfirm() {
    if (!isAdmin || !isRunning) return
    setJojaConfirmInput('')
    setJojaMessage(null)
    setJojaError(false)
    setJojaOpen(true)
  }

  function closeJojaConfirm() {
    setJojaOpen(false)
  }

  function updateJojaConfirmInput(value: string) {
    setJojaConfirmInput(value)
    setJojaMessage(null)
  }

  function fillJojaConfirmText() {
    setJojaConfirmInput(JOJA_CONFIRM_TEXT)
    setJojaMessage(null)
  }

  async function handleEnableJoja() {
    if (jojaConfirmInput !== JOJA_CONFIRM_TEXT) return
    setJojaBusy(true)
    setJojaMessage(null)
    setJojaError(false)
    try {
      await submitAndWaitForPlayerCommand(
        () => enableJojaRoute(jojaConfirmInput),
        'enable-joja',
        '',
        (feedback) => {
          setJojaError(feedback.kind === 'failed')
          setJojaMessage(feedback.message)
        },
      )
    } catch (e) {
      setJojaError(true)
      setJojaMessage(errorMessage(e))
    } finally {
      setJojaBusy(false)
    }
  }

  return {
    jojaOpen,
    jojaConfirmInput,
    jojaBusy,
    jojaMessage,
    jojaError,
    jojaConfirmText: JOJA_CONFIRM_TEXT,
    openJojaConfirm,
    closeJojaConfirm,
    updateJojaConfirmInput,
    fillJojaConfirmText,
    handleEnableJoja,
  }
}
