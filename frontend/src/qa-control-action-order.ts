// Only loaded by qa-layout.html. Synthetic touch events exercise the mounted
// component's native listeners; they do not replace physical-device testing.
export function installControlActionOrderQA() {
  const run = document.createElement('button')
  run.textContent = '运行快捷排序回归'
  run.style.cssText = 'position:fixed;top:65px;right:12px;z-index:9999;padding:10px'
  const output = document.createElement('output')
  output.id = 'qa-control-order-result'
  output.style.cssText = 'position:fixed;top:110px;left:12px;right:12px;z-index:9999;background:white;color:black;padding:10px;font-size:12px;pointer-events:none'
  output.hidden = true
  document.body.append(run, output)
  run.onclick = async () => {
    run.disabled = true
    output.hidden = false
    output.textContent = '正在验证长按、滚动与取消…'
    const wait = (ms: number) => new Promise(resolve => setTimeout(resolve, ms))
    const assert = (value: unknown, message: string) => { if (!value) throw new Error(message) }
    const rows = () => [...document.querySelectorAll<HTMLElement>('[data-control-action]')]
    const order = () => rows().map(row => row.dataset.controlAction).join(',')
    const original = rows().map(row => row.dataset.controlAction!)
    const scroll = document.querySelector<HTMLElement>('.sd-mshell-scroll')!
    const originalScroll = scroll?.scrollTop ?? 0
    const prefs = () => JSON.stringify(Object.keys(localStorage).filter(key => key.startsWith('anxipanel:control-order:')).sort().map(key => [key, localStorage.getItem(key)]))
    const originalPrefs = prefs()
    let target: HTMLElement | null = null
    const touch = (type: string, node: HTMLElement, y: number, count = 1) => {
      const rect = node.getBoundingClientRect()
      const first = new Touch({ identifier: 71, target: node, clientX: rect.left + 30, clientY: y })
      const touches = count === 0 ? [] : count === 1 ? [first] : [first, new Touch({ identifier: 72, target: node, clientX: rect.left + 40, clientY: y })]
      const event = new TouchEvent(type, { bubbles: true, cancelable: true, touches, targetTouches: touches, changedTouches: [first] })
      node.dispatchEvent(event)
      return event.defaultPrevented
    }
    const hold = async (row: HTMLElement) => {
      target = row
      row.scrollIntoView({ block: 'center' })
      await wait(50)
      const y = row.getBoundingClientRect().top + 24
      touch('touchstart', row, y)
      await wait(500)
      assert(row.classList.contains('is-dragging'), 'hold did not lift the card')
      return y
    }
    try {
      assert(original.length === 7 && scroll, 'open the mobile Control tab first')
      const row = rows()[1]
      row.scrollIntoView({ block: 'center' })
      await wait(50)
      let y = row.getBoundingClientRect().top + 24
      touch('touchstart', row, y)
      touch('touchend', row, y, 0)
      await wait(500)
      assert(!document.querySelector('.is-dragging') && order() === original.join(','), 'short tap reordered')
      touch('touchstart', row, y)
      assert(!touch('touchmove', row, y + 30), 'ordinary swipe was blocked')
      await wait(500)
      touch('touchend', row, y + 30, 0)
      assert(!document.querySelector('.is-dragging'), 'swipe became a drag')
      const action = row.querySelector<HTMLButtonElement>('button')!
      touch('touchstart', action, y)
      await wait(500)
      assert(!document.querySelector('.is-dragging'), 'action button began sorting')
      touch('touchend', action, y, 0)

      for (const cancel of ['touchcancel', 'multitouch', 'Escape', 'blur']) {
        y = await hold(row)
        assert(touch('touchmove', row, rows()[0].getBoundingClientRect().top + 10), 'active drag did not prevent scrolling')
        await wait(80)
        assert(order() !== original.join(','), 'drag did not reorder')
        if (cancel === 'multitouch') touch('touchstart', row, y, 2)
        else if (cancel === 'Escape') row.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }))
        else if (cancel === 'blur') window.dispatchEvent(new Event('blur'))
        else touch('touchcancel', row, y, 0)
        await wait(50)
        assert(order() === original.join(',') && !document.querySelector('.is-dragging'), cancel + ' did not restore order')
        assert(prefs() === originalPrefs, cancel + ' persisted a cancelled drag')
      }

      await hold(row)
      const beforeScroll = scroll.scrollTop
      const edge = document.querySelector('.sd-mshell-tabbar')!.getBoundingClientRect().top - 8
      touch('touchmove', row, edge)
      await wait(300)
      assert(scroll.scrollTop > beforeScroll, 'bottom edge did not auto-scroll')
      touch('touchcancel', row, edge, 0)
      await wait(50)
      assert(order() === original.join(','), 'auto-scroll cancellation did not restore order')

      await hold(row)
      y = rows()[0].getBoundingClientRect().top + 10
      touch('touchmove', row, y)
      await wait(80)
      touch('touchend', row, y, 0)
      await wait(50)
      assert(rows()[0] === row && !document.querySelector('.is-dragging'), 'release did not commit')
      assert(prefs() !== originalPrefs, 'release did not persist')
      const ghost = new MouseEvent('click', { bubbles: true, cancelable: true, detail: 1 })
      action.dispatchEvent(ghost)
      assert(ghost.defaultPrevented && !document.querySelector('[role="dialog"]'), 'release clicked an action')
      row.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', altKey: true, bubbles: true, cancelable: true }))
      await wait(50)
      assert(order() === original.join(','), 'keyboard move failed')
      output.textContent = 'PASS：短按、普通滑动、按钮排除、长按换序、取消回退、多指/Escape/失焦、边缘滚动、松手保存、防误点及键盘排序。'
    } catch (error) { output.textContent = 'FAIL：' + String(error) }
    finally {
      if (target) touch('touchcancel', target, 0, 0)
      for (let index = 0; index < original.length; index++) {
        const row = rows().find(item => item.dataset.controlAction === original[index])
        for (let attempt = 0; row && rows().indexOf(row) > index && attempt < original.length; attempt++) {
          row.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowUp', altKey: true, bubbles: true, cancelable: true }))
          await wait(20)
        }
      }
      await wait(50)
      if (scroll) scroll.scrollTop = originalScroll
      run.disabled = false
    }
  }
}
