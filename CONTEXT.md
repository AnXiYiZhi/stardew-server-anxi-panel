# Stardew World Operations

This context names the lifecycle states used when an administrator removes a farmhand character from a running Stardew world.

## Language

**Farmhand character**:
A non-host player character stored in one Stardew world, including its inventory, progress, cabin, and cabin contents.
_Avoid_: Account, user, player record

**Waiting deletion**:
A durable administrator request that waits for its target world to be running with no connected human players before starting deletion.
_Avoid_: Queued job, background delete

**Maintenance deletion**:
An active deletion operation that temporarily closes the world to new connections while it saves, backs up, removes, and verifies a farmhand character.
_Avoid_: Online delete, force delete

**Maintenance countdown**:
The cancelable one-minute warning period before an immediate maintenance deletion closes the world to new connections.
_Avoid_: Active deletion, grace sleep

**Join gate**:
The temporary closed state that rejects new human connections while leaving the world process available for trusted maintenance work.
_Avoid_: Server shutdown, closed save

**Destructive boundary**:
The point at which the Junimo farmhand deletion request may have removed the character and cabin from the running world.

**Safety cancellation**:
A durable stop before the destructive boundary, triggered when a player starts sleeping or the world begins day settlement.
_Avoid_: Failed save, automatic rollback

**Deletion recovery**:
A state entered when persistence or verification becomes uncertain after the destructive boundary; the join gate remains closed until an administrator resolves the operation.
_Avoid_: Failed delete, automatic rollback
