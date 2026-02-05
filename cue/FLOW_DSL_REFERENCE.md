# Flow DSL Reference

Quick reference for all Flow DSL actions in ANG.

## Repository Operations

| Action | Description | Required Fields | Optional |
|--------|-------------|-----------------|----------|
| `repo.Find` | Find entity by ID, returns nil if not found | `source`, `input`, `output` | `error` |
| `repo.Get` | Get entity by ID, expects to exist | `source`, `input`, `output` | - |
| `repo.GetForUpdate` | Get with row lock (use in tx.Block) | `source`, `input`, `output` | - |
| `repo.Save` | Persist entity (upsert) | `source`, `input` | - |
| `repo.Delete` | Remove entity by ID | `source`, `input` | - |
| `repo.List` | List entities | `source`, `output` | `method`, `input` |

```cue
// Find by ID with error handling
{action: "repo.Find", source: "User", input: "req.UserID", output: "user", error: "User not found"}

// Save entity
{action: "repo.Save", source: "Order", input: "newOrder"}

// List with custom method
{action: "repo.List", source: "Order", method: "ListByUser", input: "req.UserID", output: "orders"}
```

## Mapping & Assignment

| Action | Description | Required Fields | Optional |
|--------|-------------|-----------------|----------|
| `mapping.Assign` | Assign value to field | `to`, `value` | `declare`, `type` |
| `mapping.Map` | Map fields or create entity | - | `input`/`from`, `output`/`to`, `entity` |

```cue
// Assign UUID
{action: "mapping.Assign", to: "newOrder.ID", value: "uuid.NewString()"}

// Assign timestamp
{action: "mapping.Assign", to: "newOrder.CreatedAt", value: "time.Now().UTC().Format(time.RFC3339)"}

// Assign string literal (escape quotes!)
{action: "mapping.Assign", to: "newOrder.Status", value: "\"pending\""}

// Create new entity variable
{action: "mapping.Map", output: "newOrder", entity: "Order"}

// Copy fields from request to entity
{action: "mapping.Map", input: "req", output: "newOrder"}
```

## Control Flow

| Action | Description | Required Fields | Optional |
|--------|-------------|-----------------|----------|
| `flow.If` | Conditional execution | `condition`, `then` | `else` |
| `flow.For` | Iterate over collection | `each`, `as`, `do` | - |
| `tx.Block` | Database transaction | `do` | - |
| `flow.Block` | Group steps (no tx) | `do` | - |

```cue
// Conditional
{action: "flow.If", condition: "user.Role == \"admin\"", then: [
    {action: "mapping.Assign", to: "resp.IsAdmin", value: "true"}
], else: [
    {action: "mapping.Assign", to: "resp.IsAdmin", value: "false"}
]}

// Loop
{action: "flow.For", each: "items", as: "item", do: [
    {action: "repo.Save", source: "Item", input: "item"}
]}

// Transaction
{action: "tx.Block", do: [
    {action: "repo.Save", source: "Order", input: "order"},
    {action: "repo.Save", source: "Payment", input: "payment"}
]}
```

## Validation

| Action | Description | Required Fields | Optional |
|--------|-------------|-----------------|----------|
| `logic.Check` | Validate condition, throw error if false | `condition`, `throw` | `params` |

```cue
// Simple check
{action: "logic.Check", condition: "req.Amount > 0", throw: "Amount must be positive"}

// Status check
{action: "logic.Check", condition: "order.Status == \"draft\"", throw: "Order is not editable"}

// Ownership check
{action: "logic.Check", condition: "order.UserID == req.UserID", throw: "Access denied"}
```

## Events

| Action | Description | Required Fields | Optional |
|--------|-------------|-----------------|----------|
| `event.Publish` | Publish domain event | `name` | `payload` |
| `event.Broadcast` | Broadcast to WebSocket | `name` | `payload` |

```cue
// Publish event
{action: "event.Publish", name: "OrderCreated", payload: "domain.OrderCreated{OrderID: newOrder.ID}"}

// Broadcast to WebSocket
{action: "event.Broadcast", name: "OrderUpdated", payload: "domain.OrderUpdated{OrderID: order.ID}"}
```

## State Machine

| Action | Description | Required Fields |
|--------|-------------|-----------------|
| `fsm.Transition` | Transition entity state | `entity`, `to` |

```cue
// Transition state
{action: "fsm.Transition", entity: "order", to: "confirmed"}
```

## List Operations

| Action | Description | Required Fields |
|--------|-------------|-----------------|
| `list.Append` | Append item to slice | `to`, `item` |

```cue
// Append to response
{action: "list.Append", to: "resp.Items", item: "newItem"}
```

## Function Calls

| Action | Description | Required Fields | Optional |
|--------|-------------|-----------------|----------|
| `logic.Call` | Call custom function | `func` | `args`, `output` |

```cue
// Call function with output
{action: "logic.Call", func: "calculateTotal", args: "order.Items", output: "total"}
```

## Helper Shortcuts

Use `cue/schema/flow_helpers.cue` for common patterns:

```cue
import "github.com/strog/ang/cue/schema"

// Instead of verbose:
{action: "repo.Find", source: "User", input: "req.UserID", output: "user"}

// Use helper:
schema.#FindByID & {_entity: "User", _id: "req.UserID", _var: "user"}
```

## Auto-Completion

The compiler automatically injects before `repo.Save`:
- `ID` → `uuid.NewString()` (for variables with "new" prefix)
- `CreatedAt` → `time.Now().UTC().Format(time.RFC3339)`

## Variable Naming Conventions

| Pattern | Type | Example |
|---------|------|---------|
| `newEntity` | Value (`domain.Entity`) | `newOrder`, `newUser` |
| `existing` | Pointer (`*domain.Entity`) | from `repo.Find` |
| `entity` | Pointer (`*domain.Entity`) | from `repo.Get` |
| `items` | Slice (`[]domain.Entity`) | from `repo.List` |
