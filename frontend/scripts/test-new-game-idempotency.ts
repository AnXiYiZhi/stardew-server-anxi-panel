import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'
import { createNewGame } from '../src/api.ts'
import type { NewGameConfig } from '../src/types.ts'

const config: NewGameConfig = {
  farmName: 'Idempotent Farm',
  farmType: '0',
  farmCaveChoice: 'mushrooms',
  startingCabins: 1,
  maxPlayers: 4,
  cabinLayout: 'separate',
  profitMargin: '100',
  petBreed: 0,
  moneyMode: 'shared',
  remixedCommunityCenter: false,
  remixedMineRewards: false,
  spawnMonstersOnFarm: false,
  skipIntro: true,
  farmerName: 'Host',
}

const capturedRequests: Array<{
  input: string
  method: string | undefined
  credentials: RequestCredentials | undefined
  headers: Headers
  body: string | undefined
}> = []
const originalFetch = globalThis.fetch
try {
  globalThis.fetch = async (input, init) => {
    capturedRequests.push({
      input: String(input),
      method: init?.method,
      credentials: init?.credentials,
      headers: new Headers(init?.headers),
      body: typeof init?.body === 'string' ? init.body : undefined,
    })
    return new Response(JSON.stringify({ jobId: 'job-new-game' }), {
      status: 202,
      headers: { 'Content-Type': 'application/json' },
    })
  }

  const response = await createNewGame(config, 'request-key-1', 'farm / east')
  assert.equal(response.jobId, 'job-new-game')
} finally {
  globalThis.fetch = originalFetch
}

assert.equal(capturedRequests.length, 1)
assert.equal(capturedRequests[0].input, '/api/instances/farm%20%2F%20east/saves/custom-new-game')
assert.equal(capturedRequests[0].method, 'POST')
assert.equal(capturedRequests[0].credentials, 'include')
assert.equal(capturedRequests[0].headers.get('Idempotency-Key'), 'request-key-1')
assert.equal(capturedRequests[0].headers.get('Content-Type'), 'application/json')
assert.deepEqual(JSON.parse(capturedRequests[0].body ?? ''), config)

const savesSectionPath = fileURLToPath(new URL('../src/games/stardew/SavesSection.tsx', import.meta.url))
const savesSectionSource = readFileSync(savesSectionPath, 'utf8')
const sourceFile = ts.createSourceFile(
  savesSectionPath,
  savesSectionSource,
  ts.ScriptTarget.Latest,
  true,
  ts.ScriptKind.TSX,
)

function descendants(root: ts.Node, predicate: (node: ts.Node) => boolean) {
  const matches: ts.Node[] = []
  function visit(node: ts.Node) {
    if (predicate(node)) matches.push(node)
    ts.forEachChild(node, visit)
  }
  visit(root)
  return matches
}

const submitFunctions = descendants(
  sourceFile,
  (node) => ts.isFunctionDeclaration(node) && node.name?.text === 'handleNewGameSubmit',
) as ts.FunctionDeclaration[]
assert.equal(submitFunctions.length, 1, 'SavesSection must keep one handleNewGameSubmit state owner')

const submitFunction = submitFunctions[0]
assert.ok(submitFunction.body)
const tryStatements = submitFunction.body.statements.filter(ts.isTryStatement)
assert.equal(tryStatements.length, 1, 'new-game submission must use one success/failure boundary')

const submitTry = tryStatements[0]
const declarations = descendants(submitTry.tryBlock, ts.isVariableDeclaration) as ts.VariableDeclaration[]
const fingerprint = declarations.find((declaration) => declaration.name.getText(sourceFile) === 'fingerprint')
assert.equal(fingerprint?.initializer?.getText(sourceFile), 'JSON.stringify(cfg)')

const fingerprintGuards = submitTry.tryBlock.statements.filter(ts.isIfStatement)
assert.equal(fingerprintGuards.length, 1)
assert.equal(
  fingerprintGuards[0].expression.getText(sourceFile),
  'newGameRequestRef.current?.fingerprint !== fingerprint',
  'same config must bypass request-key generation while changed config must rotate the key',
)

const keyAssignments = descendants(
  fingerprintGuards[0].thenStatement,
  (node) => ts.isBinaryExpression(node)
    && node.operatorToken.kind === ts.SyntaxKind.EqualsToken
    && node.left.getText(sourceFile) === 'newGameRequestRef.current',
) as ts.BinaryExpression[]
assert.equal(keyAssignments.length, 1)
assert.ok(ts.isObjectLiteralExpression(keyAssignments[0].right))
assert.match(keyAssignments[0].right.getText(sourceFile), /\bfingerprint\b/)
assert.match(keyAssignments[0].right.getText(sourceFile), /requestId:\s*createNewGameRequestId\(\)/)

const sendDeclaration = declarations.find((declaration) => declaration.name.getText(sourceFile) === 'res')
assert.ok(sendDeclaration?.initializer && ts.isAwaitExpression(sendDeclaration.initializer))
assert.equal(
  sendDeclaration.initializer.expression.getText(sourceFile),
  'createNewGame(cfg, newGameRequestRef.current.requestId)',
  'retry must submit the retained key associated with the same config fingerprint',
)

const successClears = descendants(
  submitTry.tryBlock,
  (node) => ts.isBinaryExpression(node)
    && node.operatorToken.kind === ts.SyntaxKind.EqualsToken
    && node.left.getText(sourceFile) === 'newGameRequestRef.current'
    && node.right.kind === ts.SyntaxKind.NullKeyword,
) as ts.BinaryExpression[]
assert.equal(successClears.length, 1, 'successful submission must clear the request key exactly once')
assert.ok(
  successClears[0].getStart(sourceFile) > sendDeclaration.initializer.getEnd(),
  'the request key may only clear after createNewGame resolves successfully',
)

const catchWrites = submitTry.catchClause
  ? descendants(
      submitTry.catchClause,
      (node) => ts.isBinaryExpression(node)
        && node.operatorToken.kind === ts.SyntaxKind.EqualsToken
        && node.left.getText(sourceFile) === 'newGameRequestRef.current',
    )
  : []
assert.equal(catchWrites.length, 0, 'failed submission must retain the key for a same-config retry')

const requestIdFactoryCalls = descendants(
  submitFunction,
  (node) => ts.isCallExpression(node) && node.expression.getText(sourceFile) === 'createNewGameRequestId',
)
assert.equal(requestIdFactoryCalls.length, 1, 'request-key rotation must stay inside the config-change guard')

console.log('new-game idempotency frontend tests passed')
