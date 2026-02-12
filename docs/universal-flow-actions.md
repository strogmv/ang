# Universal Flow Actions — Analysis of impl_ Patterns

Extracted from dealingi-back `cue/api/impl_*.cue` (13 files, ~3500 lines of hand-written Go).
These patterns are **not dealingi-specific** and should become built-in ANG flow actions.

## 1. `auth.RequireRole` — Role-based authorization guard

**Occurrences:** 10 (impl_company.cue: ListCompanyCategories, AddCompanyCategory, RemoveCompanyCategory, SubscribeToCompany, UnsubscribeFromCompany, ListCompanySubscriptions, ListCompanyUsers, UpdateCompanyUserRole, DeactivateCompanyUser, UpdateMyCompany)

**Repeated block (15 lines x 10 copies = ~150 lines):**
```go
currentUser, err := s.UserRepo.FindByID(ctx, req.UserID)
if err != nil { return resp, err }
if currentUser == nil {
    return resp, errors.New(http.StatusUnauthorized, "Unauthorized", "user not found")
}
role := strings.ToLower(currentUser.Role)
if role != "owner" && role != "admin" {
    return resp, errors.New(http.StatusForbidden, "Forbidden", "insufficient role")
}
if currentUser.CompanyID != req.CompanyID && role != "admin" {
    return resp, errors.New(http.StatusForbidden, "Forbidden", "company mismatch")
}
```

**Proposed CUE flow action:**
```cue
{ action: "auth.RequireRole"
  userID:      "req.UserID"
  companyID:   "req.CompanyID"
  roles:       ["owner", "admin"]
  adminBypass: true          // admin can access any company
  output:      "currentUser" // optional: expose loaded user for later use
}
```

**Implementation notes:**
- Emitter generates the exact block above
- `roles` list maps to `||` checks
- `adminBypass: true` adds the `role != "admin"` company mismatch bypass
- `output` optionally declares `currentUser` variable for downstream use
- Requires: UserRepo dependency on the service (auto-injected)

---

## 2. `audit.Log` — Audit trail entry

**Occurrences:** 12 (impl_company.cue x8, impl_auth.cue x3, impl_user.cue x1)

**Repeated block (7 lines x 12 copies = ~84 lines):**
```go
if s.AuditLogRepo != nil {
    audit := &domain.AuditLog{
        ID:        uuid.NewString(),
        ActorID:   req.UserID,
        CompanyID: req.CompanyID,
        Action:    "company.category.added",
        CreatedAt: now,
    }
    _ = s.AuditLogRepo.Save(ctx, audit)
}
```

**Proposed CUE flow action:**
```cue
{ action: "audit.Log"
  actor:   "req.UserID"
  company: "req.CompanyID"
  event:   "company.category.added"
}
```

**Implementation notes:**
- Always guarded by `if s.AuditLogRepo != nil`
- Always uses `uuid.NewString()` for ID
- Always uses current timestamp
- Errors are silently ignored (`_ =`)
- Requires: AuditLogRepo dependency (auto-injected if audit.Log is used)

---

## 3. `entity.PatchNonZero` — Partial update (PATCH semantics)

**Occurrences:** 5 (impl_tender.cue: UpdateTender, UpdateTenderTemplate; impl_company.cue: UpdateMyCompany)

**Repeated pattern (1 line per field, 5-8 fields each):**
```go
if req.Title != ""       { entity.Title = req.Title }
if req.Description != "" { entity.Description = req.Description }
if req.Status != ""      { entity.Status = req.Status }
if req.StartsAt != ""    { entity.StartsAt = req.StartsAt }
if req.EndsAt != ""      { entity.EndsAt = req.EndsAt }
entity.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
```

**Proposed CUE flow action:**
```cue
{ action: "entity.PatchNonZero"
  target: "tender"
  from:   "req"
  fields: ["Title", "Description", "Status", "StartsAt", "EndsAt"]
  // For int fields, zero-check uses `> 0` instead of `!= ""`
  intFields: ["StartPrice", "BidStep"]
}
```

**Implementation notes:**
- String fields: `if req.X != "" { entity.X = req.X }`
- Int fields: `if req.X > 0 { entity.X = req.X }`
- Bool fields: always assign (or use explicit flag)
- Always sets `entity.UpdatedAt = now` at the end
- Type detection from IR entity field types

---

## 4. `list.Paginate` — In-memory pagination

**Occurrences:** 3 (impl_chat.cue: ListActiveChats, GetMessages; impl_notifications.cue: ListNotifications)

**Repeated block (12 lines x 3 copies = ~36 lines):**
```go
offset := req.Offset
if offset < 0 { offset = 0 }
limit := req.Limit
if limit <= 0 { limit = 50 }
if offset >= len(items) { offset = len(items) }
end := offset + limit
if end > len(items) { end = len(items) }
for i := offset; i < end; i++ {
    // map item to response
}
```

**Proposed CUE flow action:**
```cue
{ action: "list.Paginate"
  input:        "filtered"
  offset:       "req.Offset"
  limit:        "req.Limit"
  defaultLimit: 50
  output:       "page"      // []T slice ready for iteration
  totalOutput:  "resp.Total" // optional: set total count
}
```

**Implementation notes:**
- Boundary checks are always identical
- Some use `Page * Limit` (page-based), others use raw offset
- Variant: `pageMode: true` for page-based pagination
- Output is a sub-slice, ready for `flow.For` mapping

---

## 5. `repo.Upsert` — Find-or-create with status management

**Occurrences:** 4 (impl_company.cue: AddCompanyCategory, SubscribeToCompany, RemoveCompanyCategory, UnsubscribeFromCompany)

**Pattern — activate/create:**
```go
existing, err := s.Repo.GetByXAndY(ctx, a, b)
if err != nil { return resp, err }
now := time.Now().UTC().Format(time.RFC3339)
if existing != nil {
    if existing.Status != "active" {
        existing.Status = "active"
        existing.UpdatedAt = now
        if err := s.Repo.Save(ctx, existing); err != nil { return resp, err }
    }
} else {
    item := &domain.Entity{
        ID: uuid.NewString(), FieldA: a, FieldB: b,
        Status: "active", CreatedAt: now, UpdatedAt: now,
    }
    if err := s.Repo.Save(ctx, item); err != nil { return resp, err }
}
```

**Pattern — deactivate (reverse):**
```go
existing, err := s.Repo.GetByXAndY(ctx, a, b)
if existing != nil && existing.Status != "removed" {
    existing.Status = "removed"
    existing.UpdatedAt = now
    s.Repo.Save(ctx, existing)
}
```

**Proposed CUE flow action:**
```cue
// Activate / create
{ action: "repo.Upsert"
  entity:       "CompanyCategory"
  finder:       "GetByCompanyAndCategory"
  finderArgs:   ["req.CompanyID", "slug"]
  targetStatus: "active"
  createFields: {
      CompanyID:    "req.CompanyID"
      CategorySlug: "slug"
  }
}

// Deactivate (soft delete)
{ action: "repo.SoftDelete"
  entity:     "CompanyCategory"
  finder:     "GetByCompanyAndCategory"
  finderArgs: ["req.CompanyID", "slug"]
  status:     "removed"
}
```

---

## 6. `notify.FanOut` — Broadcast notifications to audience

**Occurrences:** 3 (impl_tender.cue: CreateTender subscription notifications, SetTenderCategories category match notifications)

**Pattern (20+ lines per occurrence):**
```go
for _, u := range users {
    if u.ID == "" { continue }
    notification := &domain.Notification{
        ID:         uuid.NewString(),
        UserID:     u.ID,
        CompanyID:  companyID,
        Title:      "New tender from subscribed company",
        Body:       "A company you follow created: " + req.Title,
        Type:       "company_tender_created",
        EntityType: "tender",
        EntityID:   tenderID,
        CreatedAt:  now,
        Data:       map[string]string{"tenderId": tenderID},
    }
    s.NotificationRepo.Save(ctx, notification)
    if s.publisher != nil {
        s.publisher.PublishNotificationCreated(ctx, domain.NotificationCreated{...all fields...})
    }
}
```

**Proposed CUE flow action:**
```cue
{ action: "notify.FanOut"
  users:      "users"          // []domain.User to iterate
  type:       "company_tender_created"
  title:      "\"New tender from subscribed company\""
  body:       "\"A company you follow created: \" + req.Title"
  entityType: "tender"
  entityID:   "tenderID"
  companyID:  "sub.FollowerCompanyID"
  data: {
      tenderId:        "tenderID"
      sourceCompanyId: "req.CompanyID"
      tenderTitle:     "req.Title"
  }
  publish: "NotificationCreated" // event to publish per notification
}
```

**Implementation notes:**
- Always skips users with empty ID
- Always creates Notification + publishes event
- Muting is already handled by MutingNotificationRepo decorator
- The NotificationCreated event mirrors all Notification fields

---

## 7. `fsm.Transition` — State machine transition validation

**Occurrences:** 2 (impl_tender.cue: UpdateAwardStatus)

**Pattern:**
```go
allowed := map[string]map[string]bool{
    "protocol_pending": {"contract_signing": true, "disputed": true},
    "contract_signing": {"docs_pending": true, "disputed": true},
    "docs_pending":     {"payment_waiting": true, "disputed": true},
    "payment_waiting":  {"fulfillment": true, "disputed": true},
    "fulfillment":      {"disputed": true},
    "disputed":         {"contract_signing": true, "fulfillment": true},
}
if !allowed[current][next] {
    return error("invalid transition")
}
entity.Status = next
```

ANG already supports `#FSM` in CUE entity definitions but does **not** generate
transition validation in service implementations. This should be automatic.

**Proposed CUE flow action:**
```cue
{ action: "fsm.Transition"
  entity: "award"
  field:  "Status"
  next:   "req.NextStatus"
  // Transitions are read from the entity's #FSM definition — no duplication
}
```

**Implementation notes:**
- ANG reads allowed transitions from CUE `#FSM.transitions`
- Generates the `allowed` map at compile time
- Returns 400 with "invalid status transition" on violation
- Optionally sets timestamp fields based on convention (`{Status}At`)

---

## Priority Matrix

| # | Action | Occurrences | Lines saved | Complexity | ROI |
|---|--------|-------------|-------------|------------|-----|
| 1 | `auth.RequireRole` | 10 | ~150 | Low | Highest |
| 2 | `audit.Log` | 12 | ~84 | Low | High |
| 3 | `entity.PatchNonZero` | 5 | ~40 | Low | High |
| 4 | `list.Paginate` | 3 | ~36 | Low | Medium |
| 5 | `repo.Upsert` | 4 | ~60 | Medium | Medium |
| 6 | `notify.FanOut` | 3 | ~60 | Medium | Medium |
| 7 | `fsm.Transition` | 2 | ~20 | Medium | Medium |

**Total potential reduction:** ~450 lines of boilerplate removed from impl_ blocks.

Items 1-4 are straightforward code generation (template substitution), implementable
in one session each. Items 5-7 require more normalizer/emitter work.

## What should stay in impl_

These patterns are **too domain-specific** for ANG:
- Auto-bet trading loop (business algorithm)
- Category tree walking with parent hierarchy
- Report PDF generation pipeline
- Chat participant management logic
- Dutch auction price drops
- Bid idempotency with transactional locking
- Weighted rating calculation
