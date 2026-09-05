// Only imported by qa-layout.html. Exercises the actual React event handlers
// against synthetic fixture data, including pointer paths unavailable in CUA.
export function installWorldDeletionQA(deleteCalls: () => number) {
  const button = document.createElement('button')
  button.textContent = '运行删除交互回归'
  button.style.cssText = 'position:fixed;bottom:12px;left:12px;z-index:9999;padding:10px'
  const output = document.createElement('output')
  output.id = 'qa-world-delete-result'
  output.style.cssText = 'position:fixed;top:60px;left:12px;right:12px;z-index:9999;background:white;color:black;padding:10px;font-size:12px'
  output.hidden = true
  document.body.append(button, output)
  button.onclick = async () => {
    button.disabled = true
    output.hidden = false
    output.textContent = '正在验证按压与取消…'
    const wait = (ms: number) => new Promise(resolve => setTimeout(resolve, ms))
    const assert = (value: unknown, message: string) => { if (!value) throw new Error(message) }
    const dispatch = (node: Element, type: string, x = 40) => node.dispatchEvent(new PointerEvent(type, { bubbles: true, pointerId: 99, pointerType: 'touch', isPrimary: true, button: 0, clientX: x, clientY: 40 }))
    try {
      const cards = document.querySelectorAll('.world-choice')
      const cover = cards[1].querySelector<HTMLButtonElement>('.world-choice-open')!
      const route = location.pathname
      for (const cancel of ['pointerup', 'pointermove', 'pointerout', 'pointercancel', 'scroll']) {
        dispatch(cover, 'pointerdown')
        await wait(240)
        assert(cards[1].querySelector('.world-delete-progress'), 'press progress missing')
        if (cancel === 'scroll') window.dispatchEvent(new Event('scroll'))
        else dispatch(cover, cancel, cancel === 'pointermove' ? 80 : 40)
        cover.dispatchEvent(new MouseEvent('click', { bubbles: true, detail: 1 }))
        await wait(1100)
        assert(!document.querySelector('dialog[open]'), cancel + ' opened dialog')
        assert(location.pathname === route, cancel + ' entered world')
      }
      const defaultCover = cards[0].querySelector('.world-choice-open')!
      dispatch(defaultCover, 'pointerdown'); await wait(1250); dispatch(defaultCover, 'pointerup')
      assert(!document.querySelector('dialog[open]'), 'default world opened dialog')
      for (const selector of ['.world-name-edit', '.world-copy-button', '.world-lifecycle-button']) {
        const child = cards[1].querySelector(selector)!
        dispatch(child, 'pointerdown'); await wait(1250); dispatch(child, 'pointerup')
        assert(!document.querySelector('dialog[open]'), selector + ' opened dialog')
      }
      dispatch(cover, 'pointerdown'); await wait(1250); dispatch(cover, 'pointerup')
      const dialog = document.querySelector<HTMLDialogElement>('dialog[open]')!
      assert(dialog, 'hold did not open confirmation')
      assert(dialog.textContent?.includes('将永久删除此世界及全部备份，无法恢复。'), 'warning missing')
      dialog.querySelector<HTMLButtonElement>('button')!.click()
      await wait(50)
      assert(!document.querySelector('dialog[open]') && document.querySelectorAll('.world-choice').length === 2, 'cancel removed world')
      assert(deleteCalls() === 0, 'cancel submitted deletion')
      dispatch(cover, 'pointerdown'); await wait(1250); dispatch(cover, 'pointerup')
      const confirm = document.querySelector<HTMLButtonElement>('.world-delete-confirm')!
      confirm.click(); confirm.click()
      await wait(50)
      assert(confirm.disabled && confirm.textContent === '删除中…', 'pending submission not locked')
      assert(deleteCalls() === 1, 'duplicate deletion submitted')
      assert(document.querySelectorAll('.world-choice').length === 2, 'world removed before server success')
      await wait(900)
      assert(document.querySelectorAll('.world-choice').length === 1 && !document.querySelector('dialog[open]'), 'successful deletion did not remove card')
      output.textContent = 'PASS：进度、提前松手、拖动、移出、pointercancel、滚动、默认保护、改名/复制/启停子控件、长按确认、取消保留、重复提交抑制、成功后移除。'
    } catch (error) { output.textContent = 'FAIL：' + String(error) }
    finally { button.disabled = false }
  }
}
