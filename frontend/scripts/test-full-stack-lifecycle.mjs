import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { createServer } from 'vite'
import { fixturePort } from './fixture-port.mjs'
import { fileURLToPath } from 'node:url'

const require = createRequire(import.meta.url)
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const entry = [
  "import React,{StrictMode,useState} from 'react'",
  "import {createRoot} from 'react-dom/client'",
  "import {useFullStackUpgrade} from '/src/games/stardew/useFullStackUpgrade.ts'",
  "import {projectFullStackApply} from '/src/games/stardew/full-stack-state.ts'",
  "import {FullStackUpgradePanel} from '/src/games/stardew/FullStackUpgradePanel.tsx'",
  "import {FixtureProvider} from '/fixture-context'",
  "import {markSessionAuthenticated} from '/src/auth-session-events.ts'",
  "import '/src/games/stardew/UpdateDetailsDialog.css'",
  "function App(){const [admin,setAdmin]=useState(window.fixture.admin),[user,setUser]=useState(41),[on,setOn]=useState(true);const data=useFullStackUpgrade(user,admin,()=>{},v=>window.fixture.offline=v,()=>window.fixture.finished++);window.fixture.confirm=data.confirmFullStack;window.fixture.recover=data.recoverFullStack;window.fixture.logout=()=>{markSessionAuthenticated();setUser(42);setAdmin(false)};window.fixture.unmount=()=>setOn(false);const projected=projectFullStackApply(null,data.fullStackUpgrade,'0.7.3');return on?React.createElement(FixtureProvider,{value:{...data,versionInfo:{version:'0.7.3',runtimeUpgrade:{requiresScopeConfirmation:true,serverVersion:'1.5.0-preview.140',authVersion:'1.5.0-anxi.3'}},updateStatus:{updateAvailable:false}}},React.createElement('output',null,JSON.stringify({status:data.fullStackUpgrade,projected})),admin?React.createElement(FullStackUpgradePanel):null):React.createElement('output',null,'unmounted')}",
  "createRoot(document.getElementById('root')).render(React.createElement(StrictMode,null,React.createElement(App)))",
].join('\n')
const contextModule = "import{createContext,useContext}from'react';const C=createContext(null);export const FixtureProvider=C.Provider;export const usePanelUpdate=()=>useContext(C)"
const server = await createServer({
  root:fileURLToPath(new URL('..',import.meta.url)),configFile:false,logLevel:'error',server:{host:'127.0.0.1',port:await fixturePort(),strictPort:true},
  plugins:[{name:'full-stack-lifecycle',enforce:'pre',resolveId(id,importer){
    if(id==='/fixture.tsx')return '\0full-stack-entry.tsx'
    if(id==='/fixture-context'||id==='./PanelUpdateProvider'&&importer?.includes('FullStackUpgradePanel'))return '\0full-stack-context'
  },load(id){if(id==='\0full-stack-entry.tsx')return entry;if(id==='\0full-stack-context')return contextModule},configureServer(s){s.middlewares.use((req,res,next)=>{
    if(req.url!=='/full-stack-test')return next()
    res.setHeader('Content-Type','text/html');res.end('<!doctype html><title>Full stack lifecycle</title><style>output{display:none}body{margin:0}</style><div id="root"></div><script type="module" src="/fixture.tsx"></script>')
  })}}],
})
await server.listen()
const url='http://127.0.0.1:'+server.httpServer.address().port+'/full-stack-test'
const browser=await chromium.launch({channel:process.env.PLAYWRIGHT_CHANNEL||'chrome',headless:true})
const results=[]
const plan=()=>({operationId:'stack_'+'a'.repeat(32),planSha256:'b'.repeat(64),actorId:41,phase:'awaiting_confirmation',panel:{version:'0.7.3',image:'candidate:local',imageId:'sha256:'+'c'.repeat(64),revision:'d'.repeat(40),buildDate:'2026-10-16T00:00:00Z'},runtimeVersion:'1.5.0-preview.140',runtimeDigest:'sha256:'+'e'.repeat(64),controlVersion:'0.3.9',controlSha256:'f'.repeat(64),gameBuildId:'17945490',sdkBuildId:'fixture-sdk',smapiIdentity:'Junimo .140 image-owned patched SMAPI',createdAt:'2026-10-16T00:00:00Z',expiresAt:'2026-10-17T00:00:00Z',continuation:true,groups:[{primaryId:'a',members:['a','b','cold'],memberNames:{a:'农场 A',b:'农场 B',cold:'空模板'},runningWorlds:2,coldTemplates:1,savedWorlds:0,stoppedWorlds:0,updatedWorlds:0,scopeSha256:'1'.repeat(64),applyId:'original-apply',phase:'prepared'}]})
const read=page=>page.locator('output').textContent().then(JSON.parse)
async function until(page,condition){const deadline=Date.now()+5000;while(!condition()){assert.ok(Date.now()<deadline,'bounded request readiness');await page.waitForTimeout(20)}}
try{
  for(const scenario of ['explicit duplicate confirmation','stale GET after confirmation','logout during POST','ordinary user','interrupted reconnect','manual recovery','scope invalidation','preflight failure','historical success']){
    const context=await browser.newContext({viewport:{width:390,height:844}})
    const page=await context.newPage(),errors=[],posts=[]
    let status=plan(),gets=0,offline=false,releaseGet,releasePost
    let holdNextGet=false
    const getGate=new Promise(resolve=>{releaseGet=resolve}),postGate=new Promise(resolve=>{releasePost=resolve})
    if(scenario==='interrupted reconnect'||scenario==='manual recovery'||scenario==='scope invalidation')status.authorizedAt='2026-10-16T00:01:00Z'
    if(scenario==='interrupted reconnect')status.phase='migrating'
    if(scenario==='manual recovery')status.phase='manual_action'
    if(scenario==='scope invalidation')status.phase='scope_changed'
    if(scenario==='historical success'){status.phase='succeeded';status.authorizedAt='2026-10-16T00:01:00Z'}
    if(scenario==='preflight failure'){status.phase='preflight_failed';status.errorCode='junimo140_auth_target_unpublished';status.error='自有 .140 SteamAuth 尚未发布，原游戏保持运行。'}
    page.on('pageerror',error=>errors.push(error.message))
    await page.addInitScript(({admin})=>{
      window.fixture={admin,finished:0,offline:false}
      const original=window.setTimeout
      window.setTimeout=(fn,ms,...args)=>original(fn,ms===1500?70:ms===5000?120:ms,...args)
    },{admin:scenario!=='ordinary user'})
    await page.route('**/api/**',async route=>{
      const request=route.request(),path=new URL(request.url()).pathname
      if(path.endsWith('/full-stack')){
        if(request.method()==='GET'){
          gets++
          const snapshot=structuredClone(status)
          if(holdNextGet){holdNextGet=false;await getGate}
          if(offline){await route.abort();return}
          await route.fulfill({json:snapshot}).catch(()=>{})
        }else{
          const body=request.postDataJSON();posts.push(body)
          if(body.action==='confirm'){
            status={...status,phase:'waiting_panel',authorizedAt:'2026-10-16T00:01:00Z'}
            if(scenario==='logout during POST')await postGate
          }else if(body.action==='recover')status={...status,phase:'recovering'}
          else if(body.action==='cancel')status={...status,phase:'failed_safe',errorCode:'administrator_cancelled'}
          await route.fulfill({status:202,json:status}).catch(()=>{})
        }
      }else if(path.endsWith('/apply'))await route.fulfill({json:{phase:'succeeded',toVersion:'0.7.3',logs:[]}})
      else await route.fulfill({json:{}})
    })
    await page.goto(url,{waitUntil:'domcontentloaded'})
    await page.waitForFunction(()=>Boolean(window.fixture.confirm))
    if(scenario==='ordinary user'){
      await page.evaluate(()=>{window.fixture.confirm();window.fixture.recover()})
      await page.waitForTimeout(250)
      assert.equal(gets,0);assert.equal(posts.length,0)
      assert.equal((await read(page)).status,null)
    }else{
      await page.waitForFunction(()=>JSON.parse(document.querySelector('output').textContent).status)
      assert.equal(posts.length,0,'displaying a plan must not authorize it')
      if(scenario==='explicit duplicate confirmation'||scenario==='stale GET after confirmation'||scenario==='logout during POST'){
        if(scenario==='stale GET after confirmation'){
          holdNextGet=true
          const before=gets
          await until(page,()=>gets!==before)
        }
        await page.evaluate(()=>{window.fixture.confirm();window.fixture.confirm()})
        await until(page,()=>posts.length!==0)
        assert.equal(posts.length,1)
        assert.deepEqual(posts[0],{action:'confirm',confirm:true,operationId:plan().operationId,planSha256:plan().planSha256})
        if(scenario==='logout during POST'){
          await page.evaluate(()=>window.fixture.logout())
          releasePost()
          await page.waitForTimeout(250)
          assert.equal((await read(page)).status,null,'old administrator response cannot enter another session')
        }else{
          await page.waitForFunction(()=>JSON.parse(document.querySelector('output').textContent).status?.authorizedAt)
          releaseGet()
          await page.waitForTimeout(180)
          assert.equal((await read(page)).status.phase,'waiting_panel','late GET cannot replace accepted confirmation')
          status={...status,phase:'migrating'}
          await page.waitForFunction(()=>JSON.parse(document.querySelector('output').textContent).status?.phase==='migrating')
          assert.equal((await read(page)).projected.fullStack.progress,0,'Panel health is not whole-operation success')
          status={...status,phase:'succeeded'}
          await page.waitForFunction(()=>JSON.parse(document.querySelector('output').textContent).status?.phase==='succeeded')
          assert.equal((await read(page)).projected.fullStack.progress,100)
          assert.equal(await page.evaluate(()=>window.fixture.finished),1)
        }
      }else if(scenario==='interrupted reconnect'){
        offline=true
        await page.waitForFunction(()=>window.fixture.offline)
        offline=false
        await page.waitForFunction(()=>!window.fixture.offline)
        await page.reload({waitUntil:'domcontentloaded'})
        await page.waitForFunction(()=>JSON.parse(document.querySelector('output').textContent).status?.phase==='migrating')
        assert.equal((await read(page)).projected.fullStack.progress,0)
        assert.equal(posts.length,0,'refresh must resume by GET without another migration request')
      }else if(scenario==='manual recovery'){
        assert.equal(await page.getByRole('button',{name:'下载并预检全栈升级'}).count(),0)
        await page.evaluate(()=>{window.fixture.recover();window.fixture.recover()})
        await page.waitForFunction(()=>JSON.parse(document.querySelector('output').textContent).status?.phase==='recovering')
        assert.equal(posts.length,1);assert.equal(posts[0].planSha256,status.planSha256)
        status={...status,phase:'failed_safe'}
        await page.waitForFunction(()=>JSON.parse(document.querySelector('output').textContent).status?.phase==='failed_safe')
        assert.equal((await read(page)).projected.fullStack.progress,0)
      }else if(scenario==='scope invalidation'){
        assert.equal(await page.getByRole('button',{name:'下载并预检全栈升级'}).count(),0)
        assert.equal(await page.getByRole('button',{name:'确认上述目标和世界范围，开始升级'}).count(),0)
        await page.getByRole('button',{name:'撤销本次授权'}).click()
        await page.waitForFunction(()=>JSON.parse(document.querySelector('output').textContent).status?.phase==='failed_safe')
        assert.deepEqual(posts,[{action:'cancel',operationId:plan().operationId}])
      }else if(scenario==='historical success'){
        await page.waitForTimeout(250)
        assert.equal(await page.evaluate(()=>window.fixture.finished),0,'past success must not reopen the update dialog on each visit')
        assert.equal((await read(page)).projected.fullStack.progress,100)
        assert.equal(posts.length,0)
      }else if(scenario==='preflight failure'){
        assert.equal(await page.getByText('自有 .140 SteamAuth 尚未发布，原游戏保持运行。',{exact:true}).count(),1)
        assert.equal(await page.getByRole('button',{name:'确认上述目标和世界范围，开始升级'}).count(),0)
      }
      const size=await page.evaluate(()=>({body:document.body.scrollWidth,root:document.documentElement.scrollWidth,width:document.documentElement.clientWidth}))
      assert.ok(size.body<=size.width&&size.root<=size.width,'mobile scope and hashes must fit viewport')
    }
    assert.deepEqual(errors,[])
    results.push({scenario,gets,posts:posts.map(body=>body.action),passed:true})
    console.log(JSON.stringify(results.at(-1)))
    await context.close()
  }
}finally{await browser.close();await server.close()}
console.log('Real React StrictMode full-stack lifecycle passed: '+results.length)
