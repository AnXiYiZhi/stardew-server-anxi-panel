import { useCallback, useEffect, useState } from 'react'
import { getCommands, runCommand } from '../../api'
import { errorMessage } from '../../core/helpers'
import type { ConsoleCommandDef } from '../../types'

type ServerConsoleOptions = {
  isRunning: boolean
}

export function useServerConsole({ isRunning }: ServerConsoleOptions) {
  const [commands, setCommands] = useState<ConsoleCommandDef[]>([])
  const [commandsLoading, setCommandsLoading] = useState(false)
  const [commandsError, setCommandsError] = useState<string | null>(null)
  const [selectedCommand, setSelectedCommand] = useState('')
  const [commandBusy, setCommandBusy] = useState(false)
  const [commandResult, setCommandResult] = useState<string | null>(null)
  const [commandError, setCommandError] = useState<string | null>(null)

  const loadCommands = useCallback(async () => {
    if (!isRunning) {
      setCommands([])
      setCommandsError(null)
      return
    }
    setCommandsLoading(true)
    setCommandsError(null)
    try {
      const res = await getCommands()
      setCommands(res.commands)
      if (res.commands.length > 0 && !selectedCommand) {
        setSelectedCommand(res.commands[0].id || res.commands[0].name)
      }
    } catch (e) {
      setCommandsError(errorMessage(e))
    } finally {
      setCommandsLoading(false)
    }
  }, [isRunning, selectedCommand])

  useEffect(() => {
    void loadCommands()
  }, [loadCommands])

  function selectCommand(commandId: string) {
    setSelectedCommand(commandId)
    setCommandResult(null)
    setCommandError(null)
  }

  async function handleRunCommand() {
    if (!selectedCommand) return
    setCommandBusy(true)
    setCommandResult(null)
    setCommandError(null)
    try {
      const res = await runCommand(selectedCommand)
      setCommandResult(res.output?.trim() || '命令已执行（无输出）')
    } catch (e) {
      setCommandError(errorMessage(e))
    } finally {
      setCommandBusy(false)
    }
  }

  return {
    commands,
    commandsLoading,
    commandsError,
    selectedCommand,
    commandBusy,
    commandResult,
    commandError,
    loadCommands,
    selectCommand,
    handleRunCommand,
  }
}
