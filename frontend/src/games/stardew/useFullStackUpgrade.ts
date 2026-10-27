import { useCallback, useEffect, useRef, useState } from 'react'
import { ApiError, cancelFullStackUpgrade, confirmFullStackUpgrade, getFullStackUpgrade, getPanelUpdateApplyStatus, prepareFullStackUpgrade, recoverFullStackUpgrade } from '../../api'
import type { FullStackUpgradeStatus, PanelUpdateApplyStatus } from '../../api'
import { currentSessionGeneration } from '../../auth-session-events'
import { errorMessage } from '../../core/helpers'
import { fullStackTerminal } from './full-stack-state'

export function useFullStackUpgrade(userId: number, isAdmin: boolean, onPanel: (value:PanelUpdateApplyStatus)=>void, onConnection: (offline:boolean)=>void, onFinished:()=>void) {
  const [status,setStatus]=useState<FullStackUpgradeStatus|null>(null)
  const [error,setError]=useState<string|null>(null)
  const [busy,setBusy]=useState(false)
  const statusRef=useRef(status)
  const generation=useRef(0)
  const acting=useRef(false)
  const finished=useRef('')
  const projection=useRef(0)
  const sessionGeneration=currentSessionGeneration()
  const callbacks=useRef({onPanel,onConnection,onFinished})
  callbacks.current={onPanel,onConnection,onFinished}

  useEffect(()=>{
    const epoch=++generation.current
    const session=currentSessionGeneration()
    const controller=new AbortController()
    let timer:ReturnType<typeof setTimeout>|undefined
    let cancelled=false
    let reading=false
    statusRef.current=null
    setStatus(null)
    setError(null)
    setBusy(false)
    acting.current=false
    if (!isAdmin) return ()=>{controller.abort();generation.current++}
    const poll=async()=>{
      if(reading || acting.current || document.visibilityState!=='visible' && !statusRef.current?.authorizedAt) {
        clearTimeout(timer)
        timer=setTimeout(()=>void poll(),1500)
        return
      }
      reading=true
      const request=++projection.current
      try {
        const value=await getFullStackUpgrade(controller.signal)
        if(cancelled || epoch!==generation.current || session!==currentSessionGeneration() || request!==projection.current)return
        const previous=statusRef.current
        statusRef.current=value
        setStatus(value)
        setError(null)
        callbacks.current.onConnection(false)
        if(value.authorizedAt && (value.panelUpdateId || !value.continuation)) {
          try {
            const panel=await getPanelUpdateApplyStatus()
            if(!cancelled && epoch===generation.current && session===currentSessionGeneration() && request===projection.current) callbacks.current.onPanel(panel)
          } catch {}
        }
        if(value.authorizedAt && (fullStackTerminal(value.phase) || value.phase==='scope_changed') && finished.current!==value.operationId+value.phase) {
          finished.current=value.operationId+value.phase
          if(value.phase!=='succeeded' || previous?.operationId===value.operationId && previous.phase!=='succeeded') callbacks.current.onFinished()
        }
      } catch(e) {
        if(cancelled || epoch!==generation.current || session!==currentSessionGeneration() || request!==projection.current)return
        if(e instanceof ApiError && e.status===404) {
          statusRef.current=null
          setStatus(null)
        } else if(statusRef.current?.authorizedAt && !fullStackTerminal(statusRef.current.phase)) callbacks.current.onConnection(true)
        else setError(errorMessage(e))
      } finally {
        reading=false
        if(!cancelled) {clearTimeout(timer);timer=setTimeout(()=>void poll(),statusRef.current?.authorizedAt && !fullStackTerminal(statusRef.current.phase) ? 1500 : 5000)}
      }
    }
    const visible=()=>{if(document.visibilityState==='visible'){clearTimeout(timer);void poll()}}
    document.addEventListener('visibilitychange',visible)
    void poll()
    return ()=>{cancelled=true;controller.abort();clearTimeout(timer);document.removeEventListener('visibilitychange',visible);generation.current++}
  },[userId,isAdmin,sessionGeneration])

  const act=useCallback(async(action:()=>Promise<FullStackUpgradeStatus>)=>{
    if(!isAdmin || acting.current)return
    acting.current=true
    const request=++projection.current
    setBusy(true)
    setError(null)
    const epoch=generation.current,session=currentSessionGeneration()
    try {
      const value=await action()
      if(epoch!==generation.current || session!==currentSessionGeneration() || request!==projection.current)return
      statusRef.current=value
      setStatus(value)
    } catch(e) {
      if(epoch===generation.current && session===currentSessionGeneration())setError(errorMessage(e))
      // An uncertain POST is reconciled by GET; it is never replayed here.
    } finally {
      if(epoch===generation.current){acting.current=false;setBusy(false)}
    }
  },[isAdmin])
  const prepare=useCallback((version:string)=>act(()=>prepareFullStackUpgrade(version)),[act])
  const confirm=useCallback((noticeDigest?: string)=>{
    const plan=statusRef.current
    if(!plan || plan.phase!=='awaiting_confirmation')return Promise.resolve()
    return act(()=>confirmFullStackUpgrade(plan, noticeDigest))
  },[act])
  const cancel=useCallback(()=>{
    const plan=statusRef.current
    return plan ? act(()=>cancelFullStackUpgrade(plan.operationId)) : Promise.resolve()
  },[act])
  const recover=useCallback(()=>{
    const plan=statusRef.current
    return plan ? act(()=>recoverFullStackUpgrade(plan)) : Promise.resolve()
  },[act])
  return {fullStackUpgrade:status,fullStackError:error,fullStackBusy:busy,prepareFullStack:prepare,confirmFullStack:confirm,cancelFullStack:cancel,recoverFullStack:recover}
}
